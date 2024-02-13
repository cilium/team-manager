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
	"reflect"
	"sort"
	"strings"

	"github.com/cilium/team-manager/pkg/comparator"
	"github.com/cilium/team-manager/pkg/config"
	"github.com/cilium/team-manager/pkg/github"
	"github.com/cilium/team-manager/pkg/slices"
	"github.com/cilium/team-manager/pkg/terminal"
	gh "github.com/google/go-github/v59/github"
	"github.com/shurcooL/githubv4"
)

func (tm *Manager) pushRepositories(ctx context.Context, force, dryRun bool, localCfg, upstreamCfg *config.Config) error {
	for repo := range localCfg.Repositories {
		err := tm.pushPermissions(ctx, force, dryRun, string(repo), localCfg, upstreamCfg)
		if err != nil {
			return fmt.Errorf("unable to sync repository %q: %w", repo, err)
		}
	}

	return nil
}

func (tm *Manager) pushTeamsConfig(ctx context.Context, force, dryRun bool, localCfg, upstreamCfg *config.Config) error {
	teamsChangedOnGH := map[string]*gh.NewTeam{}
	upstreamTeams := map[string]*gh.NewTeam{}

	for localTeamName, localTeam := range localCfg.AllTeams {
		upstreamTeam := upstreamCfg.AllTeams[localTeamName]

		if upstreamTeam == nil {
			// upstream team doesn't exist because we have created it in a previous
			// function. We can skip updating it.
			continue
		}

		changed := false
		if localTeam.Description != upstreamTeam.Description {
			cmp := comparator.CompareWithNames(localTeam.Description, upstreamTeam.Description, "local", "remote")
			fmt.Printf("Local team description config out of sync with upstream for team %q: %s\n", localTeamName, cmp)
			changed = true
		}

		if localTeam.Privacy != upstreamTeam.Privacy {
			cmp := comparator.CompareWithNames(localTeam.Privacy, upstreamTeam.Privacy, "local", "remote")
			fmt.Printf("Local team privacy config out of sync with upstream for team %q: %s\n", localTeamName, cmp)
			changed = true
		}

		if localTeam.ParentTeam != upstreamTeam.ParentTeam {
			cmp := comparator.CompareWithNames(localTeam.ParentTeam, upstreamTeam.ParentTeam, "local", "remote")
			fmt.Printf("Local team parent config out of sync with upstream for team %q: %s\n", localTeamName, cmp)
			changed = true
		}

		if !changed {
			continue
		}

		var localParentTeamID *int64
		if localTeam.ParentTeam != "" {
			parentTeam := localCfg.AllTeams[string(localTeam.ParentTeam)]
			if parentTeam == nil {
				return fmt.Errorf("parent team %q of %q not found", localTeam.ParentTeam, localTeamName)
			}
			localParentTeamID = &parentTeam.RESTID
		}
		teamsChangedOnGH[localTeamName] = &gh.NewTeam{
			Name:         localTeamName,
			Description:  &localTeam.Description,
			ParentTeamID: localParentTeamID,
			Privacy:      localTeam.Privacy.RestPrivacy(),
		}

		// Construct the upstream GitHub representation of this team so that
		// we can compare it.
		var upstreamParentTeamID *int64
		if upstreamTeam.ParentTeam != "" {
			parentTeam := upstreamCfg.AllTeams[string(localTeam.ParentTeam)]
			if parentTeam != nil {
				upstreamParentTeamID = &parentTeam.RESTID
			}
		}
		upstreamTeams[localTeamName] = &gh.NewTeam{
			Name:         localTeamName,
			Description:  &upstreamTeam.Description,
			ParentTeamID: upstreamParentTeamID,
			Privacy:      upstreamTeam.Privacy.RestPrivacy(),
		}
	}

	if len(teamsChangedOnGH) == 0 {
		return nil
	}

	cmp := comparator.CompareWithNames(teamsChangedOnGH, upstreamTeams, "local", "remote")
	fmt.Printf("Going to submit the following changes:\n%s\n", cmp)
	if dryRun {
		fmt.Printf("Skipping confirmation due to dry run. No changes will be made into GitHub\n")
		return nil
	}

	yes := force
	var err error
	if !force {
		yes, err = terminal.AskForConfirmation("Continue?")
		if err != nil {
			return err
		}
	}
	if !yes {
		return nil
	}

	for teamName, teamCfg := range teamsChangedOnGH {
		removeParent := teamCfg.ParentTeamID == nil
		_, _, err := tm.ghClient.Teams.EditTeamBySlug(ctx, tm.owner, slug(teamName), *teamCfg, removeParent)

		if err != nil {
			return fmt.Errorf("unable to update team %s: %w", teamName, err)
		}

		// Update local team with the upstream changes that we have made to
		// GitHub
		localTeam := localCfg.AllTeams[teamName]
		localTeam.Privacy = config.ParsePrivacyFromREST(teamCfg.GetPrivacy())
		localTeam.Description = teamCfg.GetDescription()

		if teamCfg.ParentTeamID != nil {
			for parentTeamName, parentTeam := range localCfg.AllTeams {
				// find the new parent
				if parentTeam.RESTID != *teamCfg.ParentTeamID {
					continue
				}
				// if parents have changed then we have to swap.
				if localTeam.ParentTeam == config.TeamOrMemberName(parentTeamName) {
					break
				}
				// delete the children from old parent
				oldParent := localCfg.AllTeams[string(localTeam.ParentTeam)]
				if oldParent != nil {
					delete(oldParent.Children, teamName)
				}

				if parentTeam.Children == nil {
					parentTeam.Children = map[string]*config.TeamConfig{}
				}

				// add it to new parent
				parentTeam.Children[teamName] = localTeam
			}
		}
	}

	return nil
}

func (tm *Manager) pushTeams(ctx context.Context, force, dryRun bool, localCfg, upstreamCfg *config.Config) error {
	type teamChange struct {
		add, remove []string
	}

	// Get a list of the teams from the local config
	localTeams := make([]string, 0, len(localCfg.AllTeams))
	for teamName := range localCfg.AllTeams {
		localTeams = append(localTeams, teamName)
	}
	sort.Strings(localTeams)

	upstreamTeams := make([]string, 0, len(upstreamCfg.AllTeams))
	for teamName := range upstreamCfg.AllTeams {
		upstreamTeams = append(upstreamTeams, teamName)
	}
	sort.Strings(upstreamTeams)

	// Update local config with upstream team IDs, if they are available.
	localCfg.UpdateTeamIDsFrom(upstreamCfg)

	if reflect.DeepEqual(localTeams, upstreamTeams) {
		return nil
	}

	teams := teamChange{}

	cmp := comparator.CompareWithNames(localTeams, upstreamTeams, "local", "remote")
	fmt.Printf("Local team config out of sync with upstream: %s\n", cmp)
	toAdd := slices.NotIn(localTeams, upstreamTeams)
	toDel := slices.NotIn(upstreamTeams, localTeams)
	teams.add = toAdd
	teams.remove = toDel

	if len(teams.add) == 0 && len(teams.remove) == 0 {
		return nil
	}

	fmt.Printf("Going to submit the following changes:\n")
	fmt.Printf("    Adding teams: %s\n", strings.Join(teams.add, ", "))
	fmt.Printf("  Removing teams: %s\n", strings.Join(teams.remove, ", "))

	if dryRun {
		fmt.Printf("Skipping confirmation due to dry run. No changes will be made into GitHub\n")
		return nil
	}
	yes := force
	var err error
	if !force {
		yes, err = terminal.AskForConfirmation("Continue?")
		if err != nil {
			return err
		}
	}
	if !yes {
		return nil
	}

	if len(teams.remove) != 0 {
		teamsToRemove := map[string]struct{}{}
		deletedTeams := map[string]struct{}{}
		for _, teamName := range teams.remove {
			teamsToRemove[teamName] = struct{}{}
		}

		for _, team := range teams.remove {
			teamToRemove, ok := upstreamCfg.AllTeams[team]
			if !ok {
				// no need to Remove teams that don't exist upstream.
				delete(teamsToRemove, team)
				continue
			}
			deletedTeams[team] = struct{}{}

			for _, team := range teams.remove {
				// If we are going to delete a parent, GitHub will delete its
				// children automatically so there's no need to also send a
				// delete API request for the children.
				if teamToRemove.IsAncestorOf(team) {
					delete(teamsToRemove, team)
					deletedTeams[team] = struct{}{}
				}
			}
		}

		var teamsToRemoveSlice []string
		for teamName := range teamsToRemove {
			teamsToRemoveSlice = append(teamsToRemoveSlice, teamName)
		}

		err := tm.RemoveOrgTeams(ctx, teamsToRemoveSlice)
		if err != nil {
			return err
		}

		// Since teams were removed from the org we need to Remove them from
		// the local config for the repository-specific settings.
		for teamName := range deletedTeams {
			for _, repo := range localCfg.Repositories {
				for permission, users := range repo {
					if !permission.IsUser() {
						repo[permission] = slices.Remove(users, config.TeamOrMemberName(teamName))
						if len(repo[permission]) == 0 {
							delete(repo, permission)
						}
					}
				}
			}
		}
	}

	if len(teams.add) != 0 {
		_, err := tm.AddTeams(ctx, localCfg, teams.add)
		if err != nil {
			return err
		}
	}

	config.SetParents(localCfg)

	return nil
}

func (tm *Manager) AddTeams(ctx context.Context, localCfg *config.Config, add []string) ([]*gh.Team, error) {
	var teamsAdded []*gh.Team

	for _, teamName := range add {

		// Check if a team was already created.
		var teamCreated bool
		for _, teamAdded := range teamsAdded {
			if teamAdded.GetName() == teamName {
				teamCreated = true
			}
		}
		if teamCreated {
			continue
		}

		team := localCfg.AllTeams[teamName]
		var parentTeamID *int64

		if team.ParentTeam != "" {
			parentTeam, ok := localCfg.AllTeams[string(team.ParentTeam)]
			if !ok {
				return nil, fmt.Errorf("parent team %q of team %q not found in local configuration", team.ParentTeam, teamName)
			}
			// If the parent ID is not set then we need to create the parent
			// first.
			if parentTeam.ID == "" {
				ghTeams, err := tm.AddTeams(ctx, localCfg, []string{string(team.ParentTeam)})
				if err != nil {
					return nil, fmt.Errorf("unable to create team %q on GH: %w", team.ParentTeam, err)
				}
				if len(ghTeams) != 1 {
					var ghCreatedTeamsName []string
					for _, ghTeam := range ghTeams {
						ghCreatedTeamsName = append(ghCreatedTeamsName, fmt.Sprintf("%q", ghTeam.GetName()))
					}
					sort.Strings(ghCreatedTeamsName)
					return nil,
						fmt.Errorf("unexpected number of GH teams created. Expected 1 (%q) got %d (%s)",
							team.ParentTeam, len(ghCreatedTeamsName), strings.Join(ghCreatedTeamsName, ", "))
				}

				for _, ghTeam := range ghTeams {
					teamsAdded = append(teamsAdded, ghTeam)
				}
			}
			parentTeamID = &parentTeam.RESTID
		}

		t, _, err := tm.ghClient.Teams.CreateTeam(ctx, tm.owner, gh.NewTeam{
			Name:         teamName,
			Description:  &team.Description,
			ParentTeamID: parentTeamID,
			Privacy:      team.Privacy.RestPrivacy(),
		})
		if err != nil {
			return nil, fmt.Errorf("unable to create team %q: %w", teamName, err)
		}
		// Populate the ID fields from upstream.
		team.ID = t.GetNodeID()
		team.RESTID = t.GetID()

		teamsAdded = append(teamsAdded, t)
	}

	return teamsAdded, nil
}

func (tm *Manager) pushTeamMembership(ctx context.Context, force, dryRun bool, localCfg, upstreamCfg *config.Config) error {
	type teamChange struct {
		add, remove []string
	}

	teamChanges := map[string]teamChange{}

	for localTeamName, localTeam := range localCfg.AllTeams {
		// Since we can't get the list of excluded members from GH we have
		// to back it up and re-added it again at the end of this for-loop.
		backExcludedMembers := localTeam.CodeReviewAssignment.ExcludedMembers
		localTeam.CodeReviewAssignment.ExcludedMembers = nil

		upstreamTeam := upstreamCfg.AllTeams[localTeamName]
		// An entire new team was added, so we will add the team members.
		if upstreamTeam == nil {
			tc := teamChange{
				add: localTeam.Members,
			}
			// When creating teams the authenticated user will become a member
			// of that team. We will need to Remove it from the team if it's not
			// meant to be added.
			if tm.AuthenticatedUser != "" {
				var memberAdded bool
				for _, membersToAdd := range localTeam.Members {
					if membersToAdd == tm.AuthenticatedUser {
						memberAdded = true
					}
				}
				if !memberAdded {
					tc.remove = []string{tm.AuthenticatedUser}
				}
			}
			teamChanges[localTeamName] = tc
		} else {
			if (len(localTeam.Members) != 0 || len(upstreamTeam.Members) != 0) &&
				!reflect.DeepEqual(localTeam.Members, upstreamTeam.Members) {
				cmp := comparator.CompareWithNames(localTeam.Members, upstreamTeam.Members, "local", "remote")
				fmt.Printf("Local team membership config out of sync with upstream: %s\n", cmp)
				toAdd := slices.NotIn(localTeam.Members, upstreamTeam.Members)
				toDel := slices.NotIn(upstreamTeam.Members, localTeam.Members)
				if len(toAdd) != 0 || len(toDel) != 0 {
					teamChanges[localTeamName] = teamChange{
						add:    toAdd,
						remove: toDel,
					}
				}
			}
		}
		localTeam.CodeReviewAssignment.ExcludedMembers = backExcludedMembers
	}

	if len(teamChanges) == 0 {
		return nil
	}

	fmt.Printf("Going to submit the following changes:\n")
	for teamName, teamCfg := range teamChanges {
		fmt.Printf(" Team: %s\n", teamName)
		fmt.Printf("    Adding members: %s\n", strings.Join(teamCfg.add, ", "))
		fmt.Printf("  Removing members: %s\n", strings.Join(teamCfg.remove, ", "))
	}
	if dryRun {
		fmt.Printf("Skipping confirmation due to dry run. No changes will be made into GitHub\n")
		return nil
	}

	yes := force
	var err error
	if !force {
		yes, err = terminal.AskForConfirmation("Continue?")
		if err != nil {
			return err
		}
	}
	if !yes {
		return nil
	}

	for teamName, teamCfg := range teamChanges {
		if err := tm.pushTeamMembers(ctx, teamName, teamCfg.add, teamCfg.remove); err != nil {
			return fmt.Errorf("unable to sync team %s: %w\n", teamName, err)
		}
		teamMembers := map[string]struct{}{}
		for _, member := range localCfg.AllTeams[teamName].Members {
			teamMembers[member] = struct{}{}
		}
		for _, rmMember := range teamCfg.remove {
			delete(teamMembers, rmMember)
		}
		for _, addMember := range teamCfg.add {
			teamMembers[addMember] = struct{}{}
		}
		team := localCfg.AllTeams[teamName]
		team.Members = make([]string, 0, len(teamMembers))
		for teamMember := range teamMembers {
			team.Members = append(team.Members, teamMember)
		}
		localCfg.AllTeams[teamName] = team
	}

	return nil
}

func (tm *Manager) pushCodeReviewAssignments(ctx context.Context, localCfg *config.Config, force bool, dryRun bool) error {
	if dryRun {
		fmt.Printf("Skipping sync of CodeReviewAssignments due to dry run. No changes will be made into GitHub\n")
		return nil
	}

	yes := force
	var err error
	if !force {
		yes, err = terminal.AskForConfirmation("Do you want to update CodeReviewAssignments?")
		if err != nil {
			return err
		}
	}
	if !yes {
		return nil
	}

	teamNames := make([]string, 0, len(localCfg.AllTeams))
	for teamName := range localCfg.AllTeams {
		teamNames = append(teamNames, teamName)
	}
	sort.Strings(teamNames)

	for _, teamName := range teamNames {
		storedTeam := localCfg.AllTeams[teamName]
		cra := storedTeam.CodeReviewAssignment
		usersIDs := getExcludedUsers(teamName, localCfg.Members, cra.ExcludedMembers, localCfg.ExcludeCRAFromAllTeams)

		input := github.UpdateTeamReviewAssignmentInput{
			Algorithm:             cra.Algorithm,
			Enabled:               githubv4.Boolean(cra.Enabled),
			ExcludedTeamMemberIDs: usersIDs,
			NotifyTeam:            githubv4.Boolean(cra.NotifyTeam),
			TeamMemberCount:       githubv4.Int(cra.TeamMemberCount),
			IncludeChildTeamMembers: func() *githubv4.Boolean {
				if cra.IncludeChildTeamMembers != nil {
					return githubv4.NewBoolean(githubv4.Boolean(*cra.IncludeChildTeamMembers))
				}
				return nil
			}(),
		}
		fmt.Printf("Excluding members from team: %s\n", teamName)
		err := tm.pushCodeReviewAssignmentForTeam(ctx, storedTeam.ID, input)
		if err != nil {
			return fmt.Errorf("unable to sync team excluded members %s: %w\n", teamName, err)
		}
	}
	return nil
}

// Sync Members
func (tm *Manager) pushMembers(ctx context.Context, force, dryRun bool, localCfg, upstreamCfg *config.Config) error {
	type membersChange struct {
		add, remove []string
	}

	// Get a list of all organization members from the local config
	var localMembers, upstreamMembers []string
	for k := range localCfg.Members {
		localMembers = append(localMembers, k)
	}
	sort.Strings(localMembers)

	// Get a list of all organization members from the upstream config
	for k, upstreamMember := range upstreamCfg.Members {
		upstreamMembers = append(upstreamMembers, k)

		localUser := localCfg.Members[k]
		if localUser.ID != upstreamMember.ID {
			localUser.ID = upstreamMember.ID
			localCfg.Members[k] = localUser
		}
	}
	sort.Strings(upstreamMembers)

	membersChanges := membersChange{}

	if reflect.DeepEqual(localMembers, upstreamMembers) {
		return nil
	}

	cmp := comparator.CompareWithNames(localMembers, upstreamMembers, "local", "remote")
	fmt.Printf("Local members config out of sync with upstream: %s\n", cmp)
	toAdd := slices.NotIn(localMembers, upstreamMembers)
	toDel := slices.NotIn(upstreamMembers, localMembers)
	membersChanges.add = toAdd
	membersChanges.remove = toDel

	if len(membersChanges.add) == 0 && len(membersChanges.remove) == 0 {
		return nil
	}

	fmt.Printf("Going to submit the following changes:\n")
	fmt.Printf("    Adding members: %s\n", strings.Join(membersChanges.add, ", "))
	fmt.Printf("  Removing members: %s\n", strings.Join(membersChanges.remove, ", "))

	if dryRun {
		fmt.Printf("Skipping confirmation due to dry run. No changes will be made into GitHub\n")
		return nil
	}
	yes := force
	var err error
	if !force {
		yes, err = terminal.AskForConfirmation("Continue?")
		if err != nil {
			return err
		}
	}
	if !yes {
		return nil
	}

	if len(membersChanges.add) != 0 {
		membersAdded, err := tm.AddOrgMembers(ctx, membersChanges.add)
		if err != nil {
			return err
		}
		// Since members were added in the org. Thus, add them in the local
		// config.
		for _, user := range membersAdded {
			localUser := localCfg.Members[user.GetLogin()]
			// If the local user already has an ID, we don't need to replace it
			// with the ID fetched from the upstream.
			if localUser.ID != "" {
				continue
			}
			localCfg.Members[user.GetLogin()] = config.User{
				ID:      user.GetNodeID(),
				Name:    user.GetName(),
				SlackID: localUser.SlackID,
			}
		}
	}

	if len(membersChanges.remove) != 0 {
		// This removes people from the organization. It does not convert them
		// to outside collaborators.
		err := tm.RemoveOrgMembers(ctx, membersChanges.remove)
		if err != nil {
			return err
		}
		// Since members were removed from the org, they were also
		// removed from teams. Thus, Remove them from the local config
		// as well in the respective teams and repositories.
		for _, member := range membersChanges.remove {
			delete(localCfg.Members, member)

			for _, team := range localCfg.AllTeams {
				team.Members = slices.Remove(team.Members, member)
				team.CodeReviewAssignment.ExcludedMembers = config.RemoveExcludedMember(team.CodeReviewAssignment.ExcludedMembers, member)
			}

			for _, repo := range localCfg.Repositories {
				for permission, users := range repo {
					if permission.IsUser() {
						repo[permission] = slices.Remove(users, config.TeamOrMemberName(member))
						if len(repo[permission]) == 0 {
							delete(repo, permission)
						}
					}
				}
			}
			localCfg.ExcludeCRAFromAllTeams = slices.Remove(localCfg.ExcludeCRAFromAllTeams, member)
		}
	}
	return nil
}

func (tm *Manager) pushPermissionsMembership(ctx context.Context, force, dryRun bool,
	repoName string, localCfg *config.Config,
	localUsers, upstreamUsers, localTeams, upstreamTeams map[config.TeamOrMemberName]config.Permission) error {

	type permChange struct {
		add, remove []string
	}

	// Check members diff
	var localUsersSlice, upstreamUsersSlice []string

	for member := range localUsers {
		localUsersSlice = append(localUsersSlice, string(member))
	}
	sort.Strings(localUsersSlice)

	for member := range upstreamUsers {
		upstreamUsersSlice = append(upstreamUsersSlice, string(member))
	}
	sort.Strings(upstreamUsersSlice)

	userPerms := permChange{}
	if !reflect.DeepEqual(localUsersSlice, upstreamUsersSlice) {
		cmp := comparator.CompareWithNames(localUsersSlice, upstreamUsersSlice, "local", "remote")
		fmt.Printf("Local members config out of sync with upstream: %s\n", cmp)
		toAdd := slices.NotIn(localUsersSlice, upstreamUsersSlice)
		toDel := slices.NotIn(upstreamUsersSlice, localUsersSlice)
		userPerms.add = toAdd
		userPerms.remove = toDel
	}

	// Check teams diff
	var localTeamsSlice, upstreamTeamsSlice []string
	for teamName := range localTeams {
		localTeamsSlice = append(localTeamsSlice, string(teamName))
	}
	sort.Strings(localTeamsSlice)

	for teamName := range upstreamTeams {
		upstreamTeamsSlice = append(upstreamTeamsSlice, string(teamName))
	}
	sort.Strings(upstreamTeamsSlice)

	teamPerms := permChange{}
	if !reflect.DeepEqual(localTeamsSlice, upstreamTeamsSlice) {
		cmp := comparator.CompareWithNames(localTeamsSlice, upstreamTeamsSlice, "local", "remote")
		fmt.Printf("Local teams config out of sync with upstream: %s\n", cmp)
		toAdd := slices.NotIn(localTeamsSlice, upstreamTeamsSlice)
		toDel := slices.NotIn(upstreamTeamsSlice, localTeamsSlice)
		teamPerms.add = toAdd
		teamPerms.remove = toDel
	}

	permissionsModified := map[config.Permission][]string{}
	for permission, localUsersOrTeams := range localCfg.Repositories[config.RepositoryName(repoName)] {
		var usersWithPermChanged []string
		for _, userOrTeam := range localUsersOrTeams {
			// Check which users had their permission changed
			if permission.IsUser() {
				perm, ok := upstreamUsers[userOrTeam]
				if ok && perm == permission {
					continue
				}
			} else {
				// Check which teams had their permission changed
				perm, ok := upstreamTeams[userOrTeam]
				if ok && perm == permission {
					continue
				}
			}
			usersWithPermChanged = append(usersWithPermChanged, string(userOrTeam))
		}
		if len(usersWithPermChanged) == 0 {
			continue
		}

		// Store the permissions that were modified in the repository.
		permissionsModified[permission] = usersWithPermChanged
		userOrTeam := "users"
		if !permission.IsUser() {
			userOrTeam = "teams"
		}

		fmt.Printf("Going to submit the following changes:\n")
		fmt.Printf("  Adding %s with permission %q to repository %q: %s\n", userOrTeam, permission, repoName, strings.Join(usersWithPermChanged, ", "))
	}
	if len(userPerms.remove) != 0 {
		fmt.Printf("Going to submit the following changes:\n")
		for _, user := range userPerms.remove {
			fmt.Printf("  Removing permission %q for user %q from repository %q\n", upstreamUsers[config.TeamOrMemberName(user)], user, repoName)
		}
	}
	if len(teamPerms.remove) != 0 {
		fmt.Printf("Going to submit the following changes:\n")
		for _, team := range teamPerms.remove {
			fmt.Printf("  Removing permission %q for team %q from repository %q\n", upstreamTeams[config.TeamOrMemberName(team)], team, repoName)
		}
	}

	if len(permissionsModified) == 0 && len(userPerms.remove) == 0 && len(teamPerms.remove) == 0 {
		return nil
	}

	if dryRun {
		fmt.Printf("Skipping confirmation due to dry run. No changes will be made into GitHub\n")
		return nil
	}
	yes := force
	var err error
	if !force {
		yes, err = terminal.AskForConfirmation("Continue?")
		if err != nil {
			return err
		}
	}
	if !yes {
		return nil
	}

	if len(userPerms.remove) != 0 {
		err := tm.PushRepositoryMembersPermissions(ctx, repoName, "", nil, userPerms.remove)
		if err != nil {
			return err
		}
	}
	if len(teamPerms.remove) != 0 {
		err = tm.PushRepositoryTeamPermissions(ctx, repoName, "", nil, teamPerms.remove)
		if err != nil {
			return err
		}
	}

	if len(permissionsModified) != 0 {
		for permission, users := range permissionsModified {
			if permission.IsUser() {
				err := tm.PushRepositoryMembersPermissions(ctx, repoName, permission.GetPermission(), users, nil)
				if err != nil {
					return err
				}
			} else {
				err = tm.PushRepositoryTeamPermissions(ctx, repoName, permission.GetPermission(), users, nil)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (tm *Manager) pushPermissions(ctx context.Context, force, dryRun bool, repoName string, localCfg, upstreamCfg *config.Config) error {
	localRepo := localCfg.Repositories[config.RepositoryName(repoName)]

	// Get all teams and users for this repository
	localUsers := map[config.TeamOrMemberName]config.Permission{}
	localTeams := map[config.TeamOrMemberName]config.Permission{}
	for localPerm, usersOrTeams := range localRepo {
		if localPerm.IsUser() {
			for _, user := range usersOrTeams {
				localUsers[user] = localPerm
			}
		} else {
			for _, team := range usersOrTeams {
				localTeams[team] = localPerm
			}
		}
	}

	upstreamRepo := upstreamCfg.Repositories[config.RepositoryName(repoName)]

	// Get all teams and users for this repository
	upstreamUsers := map[config.TeamOrMemberName]config.Permission{}
	upstreamTeams := map[config.TeamOrMemberName]config.Permission{}
	for upstreamPerm, usersOrTeams := range upstreamRepo {
		if upstreamPerm.IsUser() {
			for _, user := range usersOrTeams {
				upstreamUsers[user] = upstreamPerm
			}
		} else {
			for _, team := range usersOrTeams {
				upstreamTeams[team] = upstreamPerm
			}
		}
	}

	// Check if teams were added or removed, but not updated, in the repo.
	err := tm.pushPermissionsMembership(ctx, force, dryRun, repoName, localCfg, localUsers, upstreamUsers, localTeams, upstreamTeams)
	if err != nil {
		return fmt.Errorf("unable to sync permission membership for repo %q: %w", repoName, err)
	}
	return nil
}

// pushTeamMembers adds and removes the given login names into the given team
// name.
func (tm *Manager) pushTeamMembers(ctx context.Context, teamName string, add, remove []string) error {
	for _, user := range add {
		fmt.Printf("Adding member %s to team %s\n", user, teamName)
		if _, _, err := tm.ghClient.Teams.AddTeamMembershipBySlug(ctx, tm.owner, slug(teamName), user, &gh.TeamAddTeamMembershipOptions{Role: "member"}); err != nil {
			return err
		}
	}
	for _, user := range remove {
		fmt.Printf("Removing member %s from team %s\n", user, teamName)
		if _, err := tm.ghClient.Teams.RemoveTeamMembershipBySlug(ctx, tm.owner, slug(teamName), user); err != nil {
			return err
		}
	}
	return nil
}

// pushCodeReviewAssignmentForTeam updates the review assignment into GH for the given
// team name with the given team ID.
func (tm *Manager) pushCodeReviewAssignmentForTeam(ctx context.Context, teamID githubv4.ID, input github.UpdateTeamReviewAssignmentInput) error {
	var m struct {
		UpdateTeamReviewAssignment struct {
			Team struct {
				ID githubv4.ID
			}
		} `graphql:"updateTeamReviewAssignment(input: $input)"`
	}
	input.ID = teamID
	return tm.gqlGHClient.Mutate(ctx, &m, input, nil)
}

func (tm *Manager) AddOrgMembers(ctx context.Context, add []string) ([]*gh.User, error) {
	var membersAdded []*gh.User
	var allInvitations []*gh.Invitation
	if len(add) != 0 {
		page := 0
		for {
			invitations, resp, err := tm.ghClient.Organizations.ListPendingOrgInvitations(ctx, tm.owner, &gh.ListOptions{
				Page: page,
			})
			if err != nil {
				return nil, fmt.Errorf("unable to get the list of pending org invitations: %w", err)
			}
			allInvitations = append(allInvitations, invitations...)
			if resp.NextPage == 0 {
				break
			}
			page = resp.NextPage
		}
	}
	for _, memberName := range add {
		isInvited := false
		for _, invitation := range allInvitations {
			if invitation.GetLogin() == memberName {
				isInvited = true
				break
			}
		}
		if isInvited {
			continue
		}

		user, _, err := tm.ghClient.Users.Get(ctx, memberName)
		if err != nil {
			return nil, fmt.Errorf("unable to fetch information about user %q: %w", memberName, err)
		}

		_, _, err = tm.ghClient.Organizations.CreateOrgInvitation(ctx, tm.owner, &gh.CreateOrgInvitationOptions{
			InviteeID: user.ID,
		})
		if err != nil {
			return nil, fmt.Errorf("unable to invite user %q to the organization: %w", memberName, err)
		}
		membersAdded = append(membersAdded, user)
	}
	return membersAdded, nil
}

func (tm *Manager) RemoveOrgMembers(ctx context.Context, logins []string) error {
	for _, login := range logins {
		_, err := tm.ghClient.Organizations.RemoveMember(ctx, tm.owner, login)
		if err != nil {
			return err
		}
	}
	return nil
}

func (tm *Manager) RemoveOrgTeams(ctx context.Context, teamNames []string) error {
	for _, teamName := range teamNames {
		_, err := tm.ghClient.Teams.DeleteTeamBySlug(ctx, tm.owner, slug(teamName))
		if err != nil {
			return err
		}
	}
	return nil
}

func (tm *Manager) PushRepositoryTeamPermissions(ctx context.Context, repo string, perm string, add, remove []string) error {
	for _, team := range remove {
		fmt.Printf("Removing permissions for team %q in repo %q\n", team, repo)
		if _, err := tm.ghClient.Teams.RemoveTeamRepoBySlug(ctx, tm.owner, slug(team), tm.owner, repo); err != nil {
			fmt.Printf("[ERROR]: %s\n", err)
		}
	}
	for _, team := range add {
		fmt.Printf("Adding permission %q to team %q in repo %q\n", perm, team, repo)
		if _, err := tm.ghClient.Teams.AddTeamRepoBySlug(ctx, tm.owner, slug(team), tm.owner, repo, &gh.TeamAddTeamRepoOptions{
			Permission: config.GraphQLPerm2RestAPIPerm(perm),
		}); err != nil {
			fmt.Printf("[ERROR]: %s\n", err)
		}
	}
	return nil
}

func (tm *Manager) PushRepositoryMembersPermissions(ctx context.Context, repo, perm string, add, remove []string) error {
	for _, user := range remove {
		fmt.Printf("Removing permission for member %q in repo %q\n", user, repo)
		if _, err := tm.ghClient.Repositories.RemoveCollaborator(ctx, tm.owner, repo, user); err != nil {
			fmt.Printf("[ERROR]: %s\n", err)
		}
	}
	for _, user := range add {
		fmt.Printf("Adding permission %q to member %q in repo %q\n", perm, user, repo)
		if _, _, err := tm.ghClient.Repositories.AddCollaborator(ctx, tm.owner, repo, user, &gh.RepositoryAddCollaboratorOptions{
			Permission: config.GraphQLPerm2RestAPIPerm(perm),
		}); err != nil {
			fmt.Printf("[ERROR]: %s\n", err)
		}
	}
	return nil
}
