// Copyright 2021 Authors of Cilium
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package team

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	gh "github.com/google/go-github/v67/github"
	"github.com/schollz/progressbar/v3"
	"github.com/shurcooL/githubv4"
	"github.com/shurcooL/graphql"

	"github.com/cilium/team-manager/pkg/comparator"
	config "github.com/cilium/team-manager/pkg/config"
	"github.com/cilium/team-manager/pkg/slices"
)

type Manager struct {
	owner       string
	ghClient    *gh.Client
	gqlGHClient *githubv4.Client

	// AuthenticatedUser is the user authenticated with GH.
	AuthenticatedUser string
}

func NewManager(ghClient *gh.Client, gqlGHClient *githubv4.Client, owner string) (*Manager, error) {
	appSlug := os.Getenv("GITHUB_APP_SLUG")
	if appSlug != "" {
		// A valid GitHub App installation token is expected
		return &Manager{
			owner:             owner,
			ghClient:          ghClient,
			gqlGHClient:       gqlGHClient,
			AuthenticatedUser: appSlug,
		}, nil
	}

	// Fallback to authenticated user's information (works with PATs)
	user, _, err := ghClient.Users.Get(context.Background(), "")
	if err == nil {
		// Successfully got user, this is a PAT
		return &Manager{
			owner:             owner,
			ghClient:          ghClient,
			gqlGHClient:       gqlGHClient,
			AuthenticatedUser: user.GetLogin(),
		}, nil
	}

	return nil, fmt.Errorf("failed to authenticate with GHApp and user. User error: %v", err)
}

// PullConfiguration returns a *config.Config by querying the organization teams.
// It will not populate the excludedMembers from CodeReviewAssignments as GH
// does not provide an API of such field.
func (tm *Manager) PullConfiguration(ctx context.Context) (*config.Config, error) {
	c := &config.Config{
		Organization: tm.owner,
		Teams:        map[string]*config.TeamConfig{},
		Repositories: map[config.RepositoryName]config.Repository{},
		Members:      map[string]config.User{},
		AllTeams:     map[string]*config.TeamConfig{},
	}

	// Get all teams
	err := tm.fetchTeams(ctx, c)
	if err != nil {
		return nil, err
	}

	// Get all members
	err = tm.fetchMembers(ctx, c)
	if err != nil {
		return nil, err
	}

	// Get all repositories
	err = tm.fetchRepositories(ctx, c)
	if err != nil {
		return nil, err
	}

	err = config.SanityCheck(c)
	if err != nil {
		return nil, err
	}

	config.SortConfig(c)

	return c, nil
}
func (tm *Manager) fetchRepositories(ctx context.Context, c *config.Config) error {
	variables := map[string]interface{}{
		"collaboratorAffiliation": githubv4.CollaboratorAffiliationDirect,
	}

	resultRepos, err := tm.queryOrgRepos(ctx, variables)
	if err != nil {
		return fmt.Errorf("failed to queryOrgRepos github api: %w", err)
	}
	bar := progressbar.Default(int64(resultRepos.Organization.Repositories.TotalCount), "Fetching repositories")
	defer bar.Finish()

	requeryOrgs := false
	for {
		if requeryOrgs {
			resultRepos, err = tm.queryOrgRepos(ctx, variables)
			if err != nil {
				return fmt.Errorf("failed to requery org repositories: %w", err)
			}
			requeryOrgs = false
		}
		for _, repo := range resultRepos.Organization.Repositories.Nodes {
			repoName := config.RepositoryName(repo.Name)
			cfgRepo, ok := c.Repositories[repoName]
			if !ok {
				// fmt.Printf("Repository %q does not have any team associated with it\n", repoName)
				cfgRepo = config.Repository{}
			}

			requeryMembers := false
			for {
				// Requery of repositories shouldn't override the teams result
				innerResult := resultRepos
				if requeryMembers {
					innerResult, err = tm.queryOrgRepos(ctx, variables)
					if err != nil {
						return fmt.Errorf("failed to requery team members: %w", err)
					}
					requeryMembers = false

					// Find repo in result - especially important after requerying
					repo, err = innerResult.Organization.Repositories.WithName(repo.Name)
					if err != nil {
						return err
					}
				}

				for _, user := range repo.Collaborators.Edges {
					userPermission := config.Permission("<nil>")
					if user.Permission != nil {
						userPermission = config.Permission(*user.Permission)
						userPermission.SetUser()
					}
					cfgRepo[userPermission] = append(cfgRepo[userPermission], config.TeamOrMemberName(user.Node.Login))
				}
				if !repo.Collaborators.PageInfo.HasNextPage {
					break
				}
				requeryMembers = true
				variables["collaboratorsCursor"] = githubv4.NewString(repo.Collaborators.PageInfo.EndCursor)
			}
			// Clear the collaboratorsCursor as we are only using it when querying over collaborators
			variables["collaboratorsCursor"] = (*githubv4.String)(nil)

			c.Repositories[repoName] = cfgRepo
			bar.Add(1)
		}
		if !resultRepos.Organization.Repositories.PageInfo.HasNextPage {
			return nil
		}
		requeryOrgs = true
		variables["repositoriesWithRoleCursor"] = githubv4.NewString(resultRepos.Organization.Repositories.PageInfo.EndCursor)
	}
}

func (tm *Manager) fetchMembers(ctx context.Context, c *config.Config) error {
	variables := map[string]interface{}{}

	resultMembers, err := tm.queryOrgMembers(ctx, variables)
	if err != nil {
		return fmt.Errorf("failed to queryOrgMembers github api: %w", err)
	}

	bar := progressbar.Default(int64(resultMembers.Organization.Members.TotalCount), "Fetching members")
	defer bar.Finish()

	requeryMembers := false
	for {
		if requeryMembers {
			resultMembers, err = tm.queryOrgMembers(ctx, variables)
			if err != nil {
				return fmt.Errorf("failed to requery teams: %w", err)
			}
			requeryMembers = false
		}
		for _, member := range resultMembers.Organization.Members.Nodes {
			strLogin := string(member.Login)
			c.Members[strLogin] = config.User{
				ID:   fmt.Sprintf("%v", member.ID),
				Name: string(member.Name),
			}
			bar.Add(1)
		}
		if !resultMembers.Organization.Members.PageInfo.HasNextPage {
			return nil
		}
		requeryMembers = true
		variables["membersWithRoleCursor"] = githubv4.NewString(resultMembers.Organization.Members.PageInfo.EndCursor)
	}
}

func (tm *Manager) fetchTeams(ctx context.Context, c *config.Config) error {
	variables := map[string]interface{}{}

	resultTeams, err := tm.queryTeamsRepositories(ctx, variables)
	if err != nil {
		return fmt.Errorf("failed to queryTeamsRepositories github api: %w", err)
	}

	bar := progressbar.Default(int64(resultTeams.Organization.Teams.TotalCount), "Fetching teams")
	defer bar.Finish()
	requeryTeamsRepositories := false
	for {
		if requeryTeamsRepositories {
			resultTeams, err = tm.queryTeamsRepositories(ctx, variables)
			if err != nil {
				return fmt.Errorf("failed to requery teams: %w", err)
			}
			requeryTeamsRepositories = false
		}

		for _, t := range resultTeams.Organization.Teams.Nodes {
			requeryRepositories := false
			for {
				// Requery of repositories shouldn't override the teams result
				innerResult := resultTeams
				if requeryRepositories {
					innerResult, err = tm.queryTeamsRepositories(ctx, variables)
					if err != nil {
						return fmt.Errorf("failed to requery team members: %w", err)
					}
					requeryRepositories = false

					// Find team in result - especially important after requerying
					t, err = innerResult.Organization.Teams.WithID(t.ID)
					if err != nil {
						return err
					}
				}
				for _, repository := range t.Repositories.Edges {
					repositoryName := config.RepositoryName(repository.Node.Name)
					if repositoryName == "" {
						continue
					}
					repoCfg, ok := c.Repositories[repositoryName]
					if !ok {
						repoCfg = config.Repository{}
					}
					if repository.Permission != nil {
						repoPermission := config.Permission(*repository.Permission)
						repoCfg[repoPermission] = append(repoCfg[repoPermission], config.TeamOrMemberName(t.Name))
					}
					c.Repositories[repositoryName] = repoCfg
				}
				if !t.Repositories.PageInfo.HasNextPage {
					break
				}
				requeryRepositories = true
				variables["repositoriesCursor"] = githubv4.NewString(t.Repositories.PageInfo.EndCursor)
			}
			bar.Add(1)
			// Clear the repositoriesCursor as we are only using it when querying over repositories
			variables["repositoriesCursor"] = (*githubv4.String)(nil)
		}
		if !resultTeams.Organization.Teams.PageInfo.HasNextPage {
			break
		}
		requeryTeamsRepositories = true
		variables["teamsCursor"] = githubv4.NewString(resultTeams.Organization.Teams.PageInfo.EndCursor)
	}

	variables = map[string]interface{}{}

	resultMembers, err := tm.queryTeamsMembers(ctx, variables)
	if err != nil {
		return fmt.Errorf("failed to queryTeamsRepositories github api: %w", err)
	}

	requeryTeamsMembers := false
	for {
		if requeryTeamsMembers {
			resultMembers, err = tm.queryTeamsMembers(ctx, variables)
			if err != nil {
				return fmt.Errorf("failed to requery teams: %w", err)
			}
			requeryTeamsMembers = false
		}

		for _, t := range resultMembers.Organization.Teams.Nodes {
			strTeamName := string(t.Name)
			teamCfg, ok := c.Teams[strTeamName]
			if !ok {
				var cra config.CodeReviewAssignment
				if t.ReviewRequestDelegationEnabled {
					cra = config.CodeReviewAssignment{
						Algorithm:       config.TeamReviewAssignmentAlgorithm(t.ReviewRequestDelegationAlgorithm),
						Enabled:         bool(t.ReviewRequestDelegationEnabled),
						NotifyTeam:      bool(t.ReviewRequestDelegationNotifyTeam),
						TeamMemberCount: int(t.ReviewRequestDelegationMemberCount),
					}
				}
				teamCfg = &config.TeamConfig{
					ID:                   fmt.Sprintf("%v", t.ID),
					RESTID:               t.DatabaseId,
					Description:          string(t.Description),
					ParentTeam:           config.TeamOrMemberName(t.ParentTeam.Name),
					Privacy:              config.TeamPrivacy(t.Privacy),
					CodeReviewAssignment: cra,
				}
			}

			requeryMembers := false
			for {
				// Requery of members shouldn't override the teams result
				innerResult := resultMembers
				if requeryMembers {
					innerResult, err = tm.queryTeamsMembers(ctx, variables)
					if err != nil {
						return fmt.Errorf("failed to requery team members: %w", err)
					}
					requeryMembers = false

					// Find team in result - especially important after requerying
					t, err = innerResult.Organization.Teams.WithID(t.ID)
					if err != nil {
						return err
					}
				}
				for _, member := range t.Members.Nodes {
					strLogin := string(member.Login)
					teamCfg.Members = append(teamCfg.Members, strLogin)
				}
				sort.Strings(teamCfg.Members)
				c.Teams[strTeamName] = teamCfg
				if !t.Members.PageInfo.HasNextPage {
					break
				}
				requeryMembers = true
				variables["membersCursor"] = githubv4.NewString(t.Members.PageInfo.EndCursor)
			}
			// Clear the membersCursor as we are only using it when querying over members
			variables["membersCursor"] = (*githubv4.String)(nil)

		}
		if !resultMembers.Organization.Teams.PageInfo.HasNextPage {
			return nil
		}
		requeryTeamsMembers = true
		variables["teamsCursor"] = githubv4.NewString(resultMembers.Organization.Teams.PageInfo.EndCursor)
	}

}

func (tm *Manager) Diff(ctx context.Context, localCfg *config.Config, opts config.NormalizeOpts) (string, error) {
	// Fetch the configuration from upstream
	upstreamCfg, err := tm.PullConfiguration(ctx)
	if err != nil {
		return "", fmt.Errorf("unable to get upstream config: %w", err)
	}
	upstreamCfg.Normalize(opts)

	if localCfg.Equals(upstreamCfg) {
		return "", nil
	}

	cmp := comparator.CompareWithNames(localCfg, upstreamCfg, "local", "remote")
	return cmp, nil
}

func (tm *Manager) PushConfiguration(ctx context.Context, localCfg *config.Config, force, dryRun, pushRepos, pushMembers, pushTeams bool) (*config.Config, error) {
	// Fetch the configuration from upstream
	upstreamCfg, err := tm.PullConfiguration(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to get upstream config: %w", err)
	}

	if pushRepos {
		// Check repository sync
		err = CheckRepoSync(localCfg, upstreamCfg)
		if err != nil {
			return nil, fmt.Errorf("configuration out of sync: %w", err)
		}
	}

	if pushMembers {
		// Sync Members
		err = tm.pushMembers(ctx, force, dryRun, localCfg, upstreamCfg)
		if err != nil {
			return nil, fmt.Errorf("unable to get sync members: %w", err)
		}
	}

	if pushTeams {
		// Sync Teams
		err = tm.pushTeams(ctx, force, dryRun, localCfg, upstreamCfg)
		if err != nil {
			return nil, fmt.Errorf("unable to sync teams: %w", err)
		}

		// Sync Teams parenting / description / privacy
		err = tm.pushTeamsConfig(ctx, force, dryRun, localCfg, upstreamCfg)
		if err != nil {
			return nil, fmt.Errorf("unable to sync teams config: %w", err)
		}

		// Sync Team Membership
		err = tm.pushTeamMembership(ctx, force, dryRun, localCfg, upstreamCfg)
		if err != nil {
			return nil, fmt.Errorf("unable to get team membership: %w", err)
		}

		// Sync CodeReviewAssignments
		err = tm.pushCodeReviewAssignments(ctx, localCfg, force, dryRun)
		if err != nil {
			return nil, fmt.Errorf("unable to get sync code review assignment: %w", err)
		}
	}

	if pushRepos {
		// Sync Repos
		err = tm.pushRepositories(ctx, force, dryRun, localCfg, upstreamCfg)
		if err != nil {
			return nil, fmt.Errorf("unable to get sync repositories: %w", err)
		}
	}

	return localCfg, nil
}

func CheckRepoSync(localCfg, upstreamCfg *config.Config) error {
	type reposChange struct {
		add, remove []string
	}

	// Get a list of the repositories stored locally
	var localRepositories, upstreamRepositories []string
	for k := range localCfg.Repositories {
		localRepositories = append(localRepositories, string(k))
	}
	sort.Strings(localRepositories)

	// Get a list of the repositories from upstream
	for k := range upstreamCfg.Repositories {
		upstreamRepositories = append(upstreamRepositories, string(k))
	}
	sort.Strings(upstreamRepositories)

	// Check for changes from upstream
	repositoriesChanges := reposChange{}

	cmp := comparator.CompareWithNames(localRepositories, upstreamRepositories, "local", "remote")
	toAdd := slices.NotIn(localRepositories, upstreamRepositories)
	toDel := slices.NotIn(upstreamRepositories, localRepositories)
	repositoriesChanges.add = toAdd
	repositoriesChanges.remove = toDel

	if len(repositoriesChanges.remove) != 0 || len(repositoriesChanges.add) != 0 {
		fmt.Printf("Local repository config out of sync with upstream: %s\n", cmp)
		for _, repo := range repositoriesChanges.remove {
			localCfg.Repositories[config.RepositoryName(repo)] = upstreamCfg.Repositories[config.RepositoryName(repo)]
		}
		if len(repositoriesChanges.remove) != 0 {
			fmt.Printf("[INFO] repositories added to local configuration: %s\n", strings.Join(repositoriesChanges.remove, ","))
		}

		for _, repo := range repositoriesChanges.add {
			delete(localCfg.Repositories, config.RepositoryName(repo))
		}
		if len(repositoriesChanges.add) != 0 {
			fmt.Printf("[INFO] repositories removed from local configuration: %s\n", strings.Join(repositoriesChanges.add, ","))
		}
	}

	return nil
}

func (tm *Manager) CheckUserStatus(ctx context.Context, localCfg *config.Config) error {
	busyMembers := map[string]struct{}{}

	fmt.Printf("Found %d teams with %d unique members\n", len(localCfg.AllTeams), len(localCfg.Members))

	for member := range localCfg.Members {
		fmt.Printf("Checking status of %q\n", member)
		status, err := tm.fetchMemberLimitedAvailability(ctx, member)
		if err != nil {
			return fmt.Errorf("unable to get limited availability status for member %q: %w", member, err)
		}
		if status {
			busyMembers[member] = struct{}{}
		}
	}
	excludedMembers := map[string]struct{}{}
	for _, member := range localCfg.ExcludeCRAFromAllTeams {
		excludedMembers[member] = struct{}{}
	}

	for teamName, team := range localCfg.AllTeams {
		excludedTeamMentors := map[string]struct{}{}
		for _, xMentor := range team.Mentors {
			excludedTeamMentors[xMentor] = struct{}{}
		}

		excludedTeamMembers := map[string]struct{}{}
		for _, xMember := range team.CodeReviewAssignment.ExcludedMembers {
			excludedTeamMembers[xMember.Login] = struct{}{}
		}

		var unavailableMembers int
		for _, member := range team.Members {
			_, isBusy := busyMembers[member]
			if isBusy {
				unavailableMembers++
				continue
			}
			_, isExcluded := excludedMembers[member]
			if isExcluded {
				unavailableMembers++
				continue
			}
			_, isExcluded = excludedTeamMembers[member]
			if isExcluded {
				unavailableMembers++
				continue
			}
			_, isExcluded = excludedTeamMentors[member]
			if isExcluded {
				unavailableMembers++
				continue
			}
		}

		fmt.Printf("Team %q has the following active member ratio: %d/%d.\n", teamName, len(team.Members)-unavailableMembers, len(team.Members))

		// Warn if there teams that have less than two people to review
		if len(team.Members)-1 <= unavailableMembers {
			if len(team.Members) <= 1 && unavailableMembers == 0 {
				continue
			}
			if len(team.Members) == 2 && unavailableMembers <= 1 {
				continue
			}
			fmt.Printf("Team %q with %d members doesn't have enough reviewers:\n", teamName, len(team.Members))
			for _, member := range team.Members {
				statusString := ""
				_, isBusy := busyMembers[member]
				if isBusy {
					if len(statusString) > 0 {
						statusString = statusString + ", "
					}
					statusString = statusString + "busy"
				}
				_, isExcluded := excludedMembers[member]
				if isExcluded {
					if len(statusString) > 0 {
						statusString = statusString + ", "
					}
					statusString = statusString + "org_excluded"
				}
				_, isExcluded = excludedTeamMembers[member]
				if isExcluded {
					if len(statusString) > 0 {
						statusString = statusString + ", "
					}
					statusString = statusString + "team_excluded"
				}
				_, isExcluded = excludedTeamMentors[member]
				if isExcluded {
					if len(statusString) > 0 {
						statusString = statusString + ", "
					}
					statusString = statusString + "team_mentor"
				}
				if len(statusString) > 0 {
					fmt.Printf(" - %s - %s\n", member, statusString)
					continue
				}
				fmt.Printf(" - %s - ok\n", member)
			}
		}
	}
	return nil
}

func (tm *Manager) fetchMemberLimitedAvailability(ctx context.Context, login string) (bool, error) {
	variables := map[string]interface{}{
		"login": graphql.String(login),
	}

	resultMember, err := tm.queryOrgMemberLimitedAvailability(ctx, variables)
	if err != nil {
		return false, fmt.Errorf("failed to queryOrgMembers github api: %w", err)
	}

	return bool(resultMember.User.Status.IndicatesLimitedAvailability), nil
}

func (tm *Manager) gqlQuery(ctx context.Context, q interface{}, variables map[string]interface{}) error {
	var (
		rateLimitError      *gh.RateLimitError
		abuseRateLimitError *gh.AbuseRateLimitError
	)
	for {
		err := tm.gqlGHClient.Query(ctx, q, variables)
		if err != nil {
			switch {
			case errors.As(err, &rateLimitError):
				fmt.Printf("hit rate limit, sleeping for 30 seconds...\n")
				time.Sleep(30 * time.Second)
			case errors.As(err, &abuseRateLimitError) || strings.Contains(err.Error(), "You have exceeded a secondary rate limit"):
				fmt.Printf("hit secondary limit, sleeping for 30 seconds...\n")
				time.Sleep(30 * time.Second)
			default:
				return err
			}
			continue
		}
		return nil
	}
}
