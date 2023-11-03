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
	"strings"

	"github.com/shurcooL/githubv4"
)

type RepositoryName string

type TeamOrMemberName string

type Permission githubv4.RepositoryPermission

func (p Permission) IsUser() bool {
	return strings.HasPrefix(string(p), "USER-")
}

func (p *Permission) SetUser() {
	if p != nil {
		if !p.IsUser() {
			*p = Permission(fmt.Sprintf("USER-%s", string(*p)))
		}
	}
}

func (p Permission) GetPermission() string {
	return strings.TrimPrefix(string(p), "USER-")
}

func GraphQLPerm2RestAPIPerm(perm string) string {
	switch perm {
	case "READ":
		return "pull"
	case "TRIAGE":
		return "triage"
	case "WRITE":
		return "push"
	case "MAINTAIN":
		return "maintain"
	case "ADMIN":
		return "admin"
	}
	return ""
}

type Repository map[Permission][]TeamOrMemberName

type Config struct {
	// Organization being managed.
	Organization string `json:"organization,omitempty" yaml:"organization,omitempty"`

	// URL of the Slack workspace to which the Slack user IDs belong.
	SlackWorkspace string `json:"slackWorkspace,omitempty" yaml:"slackWorkspace,omitempty"`

	// Repositories contains the list of repositories in the organization and
	// its respective team permissions.
	Repositories map[RepositoryName]Repository `json:"repositories,omitempty" yaml:"repositories,omitempty"`

	// Members maps the github login to a User.
	Members map[string]User `json:"members,omitempty" yaml:"members,omitempty"`

	// Outside collaborators maps the github login to a User.
	Collaborators map[string]OutsideCollaborator `json:"outsideCollaborators,omitempty" yaml:"outsideCollaborators,omitempty"`

	// Teams maps the github team name to a TeamConfig.
	Teams map[string]*TeamConfig `json:"teams,omitempty" yaml:"teams,omitempty"`

	// Slice of github logins that should be excluded from all team reviews
	// assignments.
	ExcludeCRAFromAllTeams []string `json:"excludeCodeReviewAssignmentFromAllTeams" yaml:"excludeCodeReviewAssignmentFromAllTeams"`

	// AllTeams is an index of all teams in the organization
	// maps the team name to its config. GitHub doesn't allow duplicated team
	// names, so we can do safely do this.
	AllTeams map[string]*TeamConfig `json:"-" yaml:"-"`
}

func (c *Config) IndexTeams() {
	allTeams := map[string]*TeamConfig{}
	getAllTeams(c.Teams, allTeams)
	c.AllTeams = allTeams
}

func getAllTeams(teams, allTeams map[string]*TeamConfig) {
	for teamName, team := range teams {
		allTeams[teamName] = team
		getAllTeams(team.Children, allTeams)
	}
}

func (c *Config) UpdateTeamIDsFrom(newCfg *Config) {
	updateTeamIDsFrom(c.AllTeams, newCfg.AllTeams)
}

func updateTeamIDsFrom(old, newTeams map[string]*TeamConfig) {
	for newTeamName, newTeam := range newTeams {
		oldTeam := old[newTeamName]
		if oldTeam != nil && oldTeam.ID != newTeam.ID {
			oldTeam.ID = newTeam.ID
			oldTeam.RESTID = newTeam.RESTID
		}
	}
}

type TeamPrivacy githubv4.TeamPrivacy

func (tp TeamPrivacy) RestPrivacy() *string {
	switch githubv4.TeamPrivacy(strings.ToUpper(string(tp))) {
	case githubv4.TeamPrivacySecret:
		return func() *string { a := "secret"; return &a }()
	case githubv4.TeamPrivacyVisible:
		return func() *string { a := "closed"; return &a }()
	}
	return nil
}

func ParsePrivacyFromREST(restPrivacy string) TeamPrivacy {
	switch restPrivacy {
	case "secret":
		return TeamPrivacy(githubv4.TeamPrivacySecret)
	default:
		return TeamPrivacy(githubv4.TeamPrivacyVisible)
	}
}

type TeamConfig struct {
	// ID is the GitHub ID of this team.
	ID string `json:"id" yaml:"id"`

	RESTID int64 `json:"restID" yaml:"restID"`

	Description string `json:"description,omitempty" yaml:"description,omitempty"`

	// Members is a list of users that belong to this team.
	Members []string `json:"members,omitempty" yaml:"members,omitempty"`

	// CodeReviewAssignment is the code review assignment configuration of this team
	CodeReviewAssignment CodeReviewAssignment `json:"codeReviewAssignment,omitempty" yaml:"codeReviewAssignment,omitempty"`

	Privacy TeamPrivacy `json:"privacy,omitempty" yaml:"privacy,omitempty"`

	ParentTeam TeamOrMemberName `json:"-" yaml:"-"`

	Children map[string]*TeamConfig `json:"children,omitempty" yaml:"children,omitempty"`
}

func (c *TeamConfig) IsAncestorOf(child string) bool {
	for name, children := range c.Children {
		if name == child {
			return true
		}
		if children.IsAncestorOf(child) {
			return true
		}
	}
	return false
}

func (c *TeamConfig) Descendents() []string {
	var descendents []string
	for name, children := range c.Children {
		descendents = append(descendents, name)
		descendents = append(descendents, children.Descendents()...)
	}
	return descendents
}

type User struct {
	// ID is the GitHub ID of this user.
	ID string `json:"id" yaml:"id"`

	// Name is the real name of the person behind this GH account.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`

	// SlackID is the Slack user ID of the person behind this GH account.
	// The user ID can be found in the UI, under the profile of each user, under "More".
	SlackID string `json:"slackID,omitempty" yaml:"slackID,omitempty"`
}

type OutsideCollaborator struct {
	// Reason contains the reason why they are an outside collaborator.
	Reason string `json:"reason" yaml:"reason"`
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
