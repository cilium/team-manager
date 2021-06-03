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
)

type Config struct {
	// Organization being managed.
	Organization string `json:"organization,omitempty" yaml:"organization,omitempty"`

	// Members maps the github login to a User.
	Members map[string]User `json:"members,omitempty" yaml:"members,omitempty"`

	// Teams maps the github team name to a TeamConfig.
	Teams map[string]TeamConfig `json:"teams,omitempty" yaml:"teams,omitempty"`

	// Slice of github logins that should be excluded from all team reviews
	// assignments.
	ExcludeCRAFromAllTeams []string `json:"excludeCodeReviewAssignmentFromAllTeams,omitempty" yaml:"excludeCodeReviewAssignmentFromAllTeams,omitempty"`
}

type TeamConfig struct {
	// ID is the GitHub ID of this team.
	ID string `json:"id" yaml:"id"`

	// Members is a list of users that belong to this team.
	Members []string `json:"members,omitempty" yaml:"members,omitempty"`

	// CodeReviewAssignment is the code review assignment configuration of this team
	CodeReviewAssignment CodeReviewAssignment `json:"codeReviewAssignment,omitempty" yaml:"codeReviewAssignment,omitempty"`
}

type User struct {
	// ID is the GitHub ID of this user.
	ID string `json:"id" yaml:"id"`

	// Name is the real name of the person behind this GH account.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
}

type ExcludedMember struct {
	// Login the login of this GH user.
	Login string `json:"login" yaml:"login"`

	// Reason states the reason why this user is excluded from the
	// CodeReviewAssignment.
	Reason string `json:"reason" yaml:"reason"`
}

type CodeReviewAssignment struct {
	// Algorithm can only be LOAD_BALANCE or ROUND_ROBIN.
	Algorithm TeamReviewAssignmentAlgorithm `json:"algorithm,omitempty" yaml:"algorithm,omitempty"`

	// Enabled should be set to true if the CRA is enabled.
	Enabled bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`

	// ExcludedMembers contains the list of members that should not receive
	// review requests.
	ExcludedMembers []ExcludedMember `json:"excludedMembers,omitempty" yaml:"excludedMembers,omitempty"`

	// NotifyTeam will notify the entire team if assigning team members.
	NotifyTeam bool `json:"notifyTeam,omitempty" yaml:"notifyTeam,omitempty"`

	// TeamMemberCount specifies the number of team members that should be
	// assigned to review.
	TeamMemberCount int `json:"teamMemberCount,omitempty" yaml:"teamMemberCount,omitempty"`
}

type TeamReviewAssignmentAlgorithm string

const (
	TeamReviewAssignmentAlgorithmLoadBalance TeamReviewAssignmentAlgorithm = "LOAD_BALANCE"
	TeamReviewAssignmentAlgorithmRoundRobin  TeamReviewAssignmentAlgorithm = "ROUND_ROBIN"
)

// SanityCheck checks if the all team members belong to the organization.
func SanityCheck(cfg *Config) error {
	// Check if all users in the CodeReviewAssignment belong to the list of
	// members
	for teamName, team := range cfg.Teams {
		for _, member := range team.Members {
			if _, ok := cfg.Members[member]; !ok {
				return fmt.Errorf("member %q from team %q does not belong to organization", member, teamName)
			}
		}
		for _, xMember := range team.CodeReviewAssignment.ExcludedMembers {
			if _, ok := cfg.Members[xMember.Login]; !ok {
				return fmt.Errorf("member %q from code review assignment of team %q does not belong to organization", xMember.Login, teamName)
			}
		}
	}
	return nil
}
