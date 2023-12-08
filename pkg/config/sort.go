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

package config

import (
	"fmt"
	"sort"

	"github.com/shurcooL/githubv4"
)

func SortConfig(cfg *Config) {
	// Index all teams in a single map.
	cfg.IndexTeams()

	for teamName := range cfg.AllTeams {
		team := cfg.AllTeams[teamName]

		// Remove and sort and duplicated team members
		teamMembers := make(map[string]struct{}, len(team.Members))
		for _, teamMember := range team.Members {
			teamMembers[teamMember] = struct{}{}
		}

		team.Members = make([]string, 0, len(teamMembers))
		for teamMember := range teamMembers {
			team.Members = append(team.Members, teamMember)
		}
		sort.Strings(team.Members)

		// sort excluded members as well
		sort.Slice(team.CodeReviewAssignment.ExcludedMembers, func(i, j int) bool {
			return team.CodeReviewAssignment.ExcludedMembers[i].Login <
				team.CodeReviewAssignment.ExcludedMembers[j].Login
		},
		)

		// If it's a parent team and the privacy is not set then default to
		// "secret".
		if team.ParentTeam == "" {
			if team.Privacy == "" {
				team.Privacy = TeamPrivacy(githubv4.TeamPrivacySecret)
			}
		} else {
			// If it's a child team and the privacy is not set then default to
			// "public"
			if team.Privacy == "" {
				team.Privacy = TeamPrivacy(githubv4.TeamPrivacyVisible)
			}
		}

		cfg.AllTeams[teamName] = team
	}
	// Sort excluded team members
	sort.Strings(cfg.ExcludeCRAFromAllTeams)

	// Set the right children of the parent teams
	SetParents(cfg)

	// Sort repositories
	for repoName, permissions := range cfg.Repositories {
		for permission, members := range permissions {
			// Remove and sort duplicated teams or members
			permMembers := make(map[TeamOrMemberName]struct{}, len(members))
			for _, teamMember := range members {
				permMembers[teamMember] = struct{}{}
			}
			// If a parent team has the same permissions then we can remove
			// any children of it.
			if !permission.IsUser() {
				for permMember := range permMembers {
					team := cfg.AllTeams[string(permMember)]
					if team == nil {
						panic(fmt.Sprintf("Couldn't find team %q", string(permMember)))
					}
					descendents := team.Descendents()
					for _, descendent := range descendents {
						delete(permMembers, TeamOrMemberName(descendent))
					}
				}
			}

			dedupMembers := make([]TeamOrMemberName, 0, len(permMembers))
			for teamMember := range permMembers {
				dedupMembers = append(dedupMembers, teamMember)
			}
			sort.Slice(dedupMembers, func(i, j int) bool {
				return dedupMembers[i] < dedupMembers[j]
			})

			permissions[permission] = dedupMembers

		}

		cfg.Repositories[repoName] = permissions
	}

	allCollaborators := map[string]struct{}{}
	for _, permissions := range cfg.Repositories {
		for permission, users := range permissions {
			if permission.IsUser() {
				for _, user := range users {
					allCollaborators[string(user)] = struct{}{}
				}
			}
		}
	}
	for member := range cfg.Members {
		delete(allCollaborators, member)
	}
	if cfg.Collaborators == nil {
		cfg.Collaborators = map[string]OutsideCollaborator{}
	}
	for outsideCollab := range cfg.Collaborators {
		// If they aren't outside collaborators, then remove them from the
		// config file.
		_, ok := allCollaborators[outsideCollab]
		if !ok {
			delete(cfg.Collaborators, outsideCollab)
		}
	}
	for collaborator := range allCollaborators {
		_, ok := cfg.Collaborators[collaborator]
		if !ok {
			cfg.Collaborators[collaborator] = OutsideCollaborator{}
		}
	}
}

func SetParents(localCfg *Config) {
	localCfg.IndexTeams()

	// Clear all children
	for _, team := range localCfg.AllTeams {
		team.Children = nil
	}

	for teamName, team := range localCfg.AllTeams {
		parentName := team.ParentTeam
		if parentName == "" {
			continue
		}
		parent := localCfg.AllTeams[string(parentName)]
		if parent == nil {
			// If it doesn't have a parent, then it's in the root.
			continue
		}
		if parent.Children == nil {
			parent.Children = map[string]*TeamConfig{}
		}
		parent.Children[teamName] = team

		// Remove the team from "root" because it is set as a child of another
		// team
		delete(localCfg.Teams, teamName)
	}
}

func SetParentNames(teams map[string]*TeamConfig) {
	for teamName, team := range teams {
		for _, child := range team.Children {
			child.ParentTeam = TeamOrMemberName(teamName)
		}
	}
}
