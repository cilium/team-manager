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

	"github.com/shurcooL/githubv4"
)

// SanityCheck checks if the all team members belong to the organization.
func SanityCheck(cfg *Config) error {
	// Check if all users in the CodeReviewAssignment belong to the list of
	// members
	for teamName, team := range cfg.AllTeams {
		for _, member := range team.Members {
			if _, ok := cfg.Members[member]; !ok {
				return fmt.Errorf("member %q from team %q does not belong to organization", member, teamName)
			}
		}
		for _, mentor := range team.Mentors {
			if _, ok := cfg.Members[mentor]; !ok {
				return fmt.Errorf("mentor %q from team %q does not belong to organization", mentor, teamName)
			}
		}
		for _, xMember := range team.CodeReviewAssignment.ExcludedMembers {
			if _, ok := cfg.Members[xMember.Login]; !ok {
				return fmt.Errorf("member %q from code review assignment of team %q does not belong to organization", xMember.Login, teamName)
			}
		}

		if team.ParentTeam != "" && githubv4.TeamPrivacy(team.Privacy) == githubv4.TeamPrivacySecret {
			return fmt.Errorf("error in team %q: child teams can't be secret", teamName)
		}
	}
	for _, xMember := range cfg.ExcludeCRAFromAllTeams {
		if _, ok := cfg.Members[xMember]; !ok {
			return fmt.Errorf("member %q from globally excluded reviews, does not belong to the organization", xMember)
		}
	}
	return nil
}
