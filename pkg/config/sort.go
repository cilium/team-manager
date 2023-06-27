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
	"sort"
)

func SortConfig(cfg *Config) {
	for teamName := range cfg.Teams {
		team := cfg.Teams[teamName]

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

		cfg.Teams[teamName] = team
	}
	// Sort excluded team members
	sort.Strings(cfg.ExcludeCRAFromAllTeams)
}
