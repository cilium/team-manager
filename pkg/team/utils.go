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
	"fmt"
	"regexp"
	"strings"

	"github.com/cilium/team-manager/pkg/config"

	"github.com/shurcooL/githubv4"
)

// getExcludedUsers returns a list of all users that should be excluded for the
// given team.
func getExcludedUsers(teamName string, members map[string]config.User, mentors []string, excTeamMembers []config.ExcludedMember, excAllTeams []string) []githubv4.ID {
	m := make(map[githubv4.ID]struct{}, len(members)+len(excTeamMembers)+len(excAllTeams))
	for _, member := range mentors {
		user, ok := members[member]
		if !ok {
			fmt.Printf("[ERROR] mentor %q from team %s, not found in the list of team members in the organization\n", member, teamName)
			continue
		}
		m[user.ID] = struct{}{}
	}
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

// slug returns the slug version of the team name. This simply replaces all
// characters that are not in the following regex `[^a-z0-9]+` with a `-`.
// It's a simplistic versions of the official's GitHub slug transformation since
// GitHub changes accents characters as well, for example 'ä' to 'a'.
func slug(s string) string {
	s = strings.ToLower(s)

	re := regexp.MustCompile("[^a-z0-9]+")
	s = re.ReplaceAllString(s, "-")

	s = strings.Trim(s, "-")
	return s
}
