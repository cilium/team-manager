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
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"

	"github.com/cilium/team-manager/pkg/comparator"
	"github.com/cilium/team-manager/pkg/config"
	"github.com/cilium/team-manager/pkg/github"
	"github.com/cilium/team-manager/pkg/slices"
	"github.com/cilium/team-manager/pkg/terminal"

	gh "github.com/google/go-github/v33/github"
	"github.com/shurcooL/githubv4"
)

type Manager struct {
	owner       string
	ghClient    *gh.Client
	gqlGHClient *githubv4.Client
}

func NewManager(ghClient *gh.Client, gqlGHClient *githubv4.Client, owner string) *Manager {
	return &Manager{
		owner:       owner,
		ghClient:    ghClient,
		gqlGHClient: gqlGHClient,
	}
}

// GetCurrentConfig returns a *config.Config by querying the organization teams.
// It will not populate the excludedMembers from CodeReviewAssignments as GH
// does not provide an API of such field.
func (tm *Manager) GetCurrentConfig(ctx context.Context) (*config.Config, error) {
	// {
	//  organization(login: "cilium") {
	//    teams(first: 100) {
	//      nodes {
	//        members(first: 100) {
	//          nodes {
	//            id
	//            login
	//          }
	//        }
	//      }
	//    }
	//  }
	// }
	var q struct {
		Organization struct {
			Teams struct {
				Nodes []struct {
					Members struct {
						Nodes []struct {
							ID    githubv4.ID
							Login githubv4.String
							Name  githubv4.String
						}
						PageInfo struct {
							EndCursor   githubv4.String
							HasNextPage githubv4.Boolean
						}
					} `graphql:"members(first: 100, after: $membersCursor)"`
					ID                                 githubv4.ID
					DatabaseID                         githubv4.Int
					Name                               githubv4.String
					ReviewRequestDelegationEnabled     githubv4.Boolean
					ReviewRequestDelegationAlgorithm   githubv4.String
					ReviewRequestDelegationMemberCount githubv4.Int
					ReviewRequestDelegationNotifyTeam  githubv4.Boolean
				}
				PageInfo struct {
					EndCursor   githubv4.String
					HasNextPage githubv4.Boolean
				}
			} `graphql:"teams(first: 100, after: $teamsCursor)"`
		} `graphql:"organization(login: $repositoryOwner)"`
	}
	variables := map[string]interface{}{
		"repositoryOwner": githubv4.String(tm.owner),
		"teamsCursor":     (*githubv4.String)(nil), // Null after argument to get first page.
		"membersCursor":   (*githubv4.String)(nil), // Null after argument to get first page.
	}
	c := &config.Config{
		Organization: tm.owner,
		Teams:        map[string]config.TeamConfig{},
		Members:      map[string]config.User{},
	}
	for {
	reQuery:
		err := tm.gqlGHClient.Query(ctx, &q, variables)
		if err != nil {
			return nil, err
		}
		for _, team := range q.Organization.Teams.Nodes {
			strTeamName := string(team.Name)
			teamCfg, ok := c.Teams[strTeamName]
			if !ok {
				var cra config.CodeReviewAssignment
				if team.ReviewRequestDelegationEnabled {
					cra = config.CodeReviewAssignment{
						Algorithm:       config.TeamReviewAssignmentAlgorithm(team.ReviewRequestDelegationAlgorithm),
						Enabled:         bool(team.ReviewRequestDelegationEnabled),
						NotifyTeam:      bool(team.ReviewRequestDelegationNotifyTeam),
						TeamMemberCount: int(team.ReviewRequestDelegationMemberCount),
					}
				}
				teamCfg = config.TeamConfig{
					ID:                   fmt.Sprintf("%v", team.ID),
					CodeReviewAssignment: cra,
				}
			}
			for _, member := range team.Members.Nodes {
				strLogin := string(member.Login)
				teamCfg.Members = append(teamCfg.Members, strLogin)
				c.Members[strLogin] = config.User{
					ID:   fmt.Sprintf("%v", member.ID),
					Name: string(member.Name),
				}
				sort.Slice(teamCfg.Members, func(i, j int) bool {
					return teamCfg.Members[i] < teamCfg.Members[j]
				})
			}
			c.Teams[strTeamName] = teamCfg
			if !team.Members.PageInfo.HasNextPage {
				continue
			}
			variables["membersCursor"] = githubv4.NewString(team.Members.PageInfo.EndCursor)
			goto reQuery
		}
		if !q.Organization.Teams.PageInfo.HasNextPage {
			break
		}
		variables["teamsCursor"] = githubv4.NewString(q.Organization.Teams.PageInfo.EndCursor)
		// Clear the membersCursor as we are only using it when querying over members
		variables["membersCursor"] = (*githubv4.String)(nil)
	}
	return c, nil
}

// SyncTeamMembers adds and removes the given login names into the given team
// name.
func (tm *Manager) SyncTeamMembers(ctx context.Context, teamName string, add, remove []string) error {
	for _, user := range add {
		fmt.Printf("Adding member %s to team %s\n", user, teamName)
		_, _, err := tm.ghClient.Teams.AddTeamMembershipBySlug(ctx, tm.owner, teamName, user, &gh.TeamAddTeamMembershipOptions{Role: "member"})
		if err != nil {
			return err
		}
	}
	for _, user := range remove {
		fmt.Printf("Removing member %s from team %s\n", user, teamName)
		_, err := tm.ghClient.Teams.RemoveTeamMembershipBySlug(ctx, tm.owner, teamName, user)
		if err != nil {
			return err
		}
	}
	return nil
}

// SyncTeamReviewAssignment updates the review assignment into GH for the given
// team name with the given team ID.
func (tm *Manager) SyncTeamReviewAssignment(ctx context.Context, teamName string, teamID githubv4.ID, input github.UpdateTeamReviewAssignmentInput) error {
	var m struct {
		UpdateTeamReviewAssignment struct {
			Team struct {
				ID githubv4.ID
			}
		} `graphql:"updateTeamReviewAssignment(input: $input)"`
	}
	input.ID = teamID
	fmt.Printf("Excluding members from team: %s\n", teamName)
	return tm.gqlGHClient.Mutate(ctx, &m, input, nil)
}

func (tm *Manager) SyncTeams(ctx context.Context, localCfg *config.Config, force bool) (*config.Config, error) {
	upstreamCfg, err := tm.GetCurrentConfig(ctx)
	if err != nil {
		return nil, err
	}

	type teamChange struct {
		add, remove []string
	}
	teamChanges := map[string]teamChange{}

	for localTeamName, localTeam := range localCfg.Teams {
		// Since we can't get the list of excluded members from GH we have
		// to back it up and re-added it again at the end of this for-loop.
		backExcludedMembers := localTeam.CodeReviewAssignment.ExcludedMembers

		localTeam.CodeReviewAssignment.ExcludedMembers = nil
		if !reflect.DeepEqual(localTeam, upstreamCfg.Teams[localTeamName]) {
			cmp := comparator.CompareWithNames(localTeam, upstreamCfg.Teams[localTeamName], "local", "remote")
			fmt.Printf("Local config out of sync with upstream: %s\n", cmp)
			toAdd := slices.NotIn(localTeam.Members, upstreamCfg.Teams[localTeamName].Members)
			toDel := slices.NotIn(upstreamCfg.Teams[localTeamName].Members, localTeam.Members)
			if len(toAdd) != 0 || len(toDel) != 0 {
				teamChanges[localTeamName] = teamChange{
					add:    toAdd,
					remove: toDel,
				}
			}
		}
		localTeam.CodeReviewAssignment.ExcludedMembers = backExcludedMembers
	}

	if len(teamChanges) != 0 {
		fmt.Printf("Going to submit the following changes:\n")
		for teamName, teamCfg := range teamChanges {
			fmt.Printf(" Team: %s\n", teamName)
			fmt.Printf("    Adding members: %s\n", strings.Join(teamCfg.add, ", "))
			fmt.Printf("  Removing members: %s\n", strings.Join(teamCfg.remove, ", "))
		}
		yes := force
		if !force {
			yes, err = terminal.AskForConfirmation("Continue?")
			if err != nil {
				return nil, err
			}
		}
		if yes {
			for teamName, teamCfg := range teamChanges {
				err = tm.SyncTeamMembers(ctx, teamName, teamCfg.add, teamCfg.remove)
				if err != nil {
					fmt.Fprintf(os.Stderr, "[ERROR]:  Unable to sync team %s: %s\n", teamName, err)
					continue
				}
				teamMembers := map[string]struct{}{}
				for _, member := range localCfg.Teams[teamName].Members {
					teamMembers[member] = struct{}{}
				}
				for _, rmMember := range teamCfg.remove {
					delete(teamMembers, rmMember)
				}
				for _, addMember := range teamCfg.add {
					teamMembers[addMember] = struct{}{}
				}
				team := localCfg.Teams[teamName]
				team.Members = make([]string, 0, len(teamMembers))
				for teamMember := range teamMembers {
					team.Members = append(team.Members, teamMember)
				}
				localCfg.Teams[teamName] = team
			}
		}
	}

	yes := force
	if !force {
		yes, err = terminal.AskForConfirmation("Do you want to update CodeReviewAssignments?")
		if err != nil {
			return nil, err
		}
	}
	if yes {
		teamNames := make([]string, 0, len(localCfg.Teams))
		for teamName := range localCfg.Teams {
			teamNames = append(teamNames, teamName)
		}
		sort.Strings(teamNames)
		for _, teamName := range teamNames {
			storedTeam := localCfg.Teams[teamName]
			cra := storedTeam.CodeReviewAssignment
			usersIDs := getExcludedUsers(teamName, localCfg.Members, cra.ExcludedMembers, localCfg.ExcludeCRAFromAllTeams)

			input := github.UpdateTeamReviewAssignmentInput{
				Algorithm:             cra.Algorithm,
				Enabled:               githubv4.Boolean(cra.Enabled),
				ExcludedTeamMemberIDs: usersIDs,
				NotifyTeam:            githubv4.Boolean(cra.NotifyTeam),
				TeamMemberCount:       githubv4.Int(cra.TeamMemberCount),
			}
			err := tm.SyncTeamReviewAssignment(ctx, teamName, storedTeam.ID, input)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[ERROR]: Unable to sync team excluded members %s: %s\n", teamName, err)
				continue
			}
		}
	}

	return localCfg, nil
}

// getExcludedUsers returns a list of all users that should be excluded for the
// given team.
func getExcludedUsers(teamName string, members map[string]config.User, excTeamMembers []config.ExcludedMember, excAllTeams []string) []githubv4.ID {
	m := make(map[githubv4.ID]struct{}, len(excTeamMembers)+len(excAllTeams))
	for _, member := range excTeamMembers {
		user, ok := members[member.Login]
		if !ok {
			fmt.Printf("[ERROR] user %q from team %s, not found in the list of team members in the organization\n", member.Login, teamName)
			continue
		}
		m[user.ID] = struct{}{}
	}
	for _, member := range excAllTeams {
		user, ok := members[member]
		if !ok {
			// Ignore if it doesn't belong to the team
			continue
		}
		m[user.ID] = struct{}{}
	}

	memberIDs := make([]githubv4.ID, 0, len(m))
	for memberID := range m {
		memberIDs = append(memberIDs, memberID)
	}
	return memberIDs
}
