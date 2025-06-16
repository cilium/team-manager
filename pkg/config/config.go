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
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"slices"
	"sort"
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

	// OverrideTeams is an index of teams in the override file
	// maps the team name to its config. GitHub doesn't allow duplicated team
	// names, so we can do safely do this.
	TeamOverrides map[string]*OverrideTeamConfig `json:"-" yaml:"-"`
}

// This will hold the information from the Team Override File
type OverrideConfig struct {
	// Teams maps the github team name to a OverrideTeamConfig.
	Teams map[string]*OverrideTeamConfig `json:"teams,omitempty" yaml:"teams,omitempty"`
}

// OverrideTeamConfig is intended to be made public containing only github usernames and team names
type OverrideTeamConfig struct {
	// Members is a list of users that belong to this team.
	Members []string `json:"members,omitempty" yaml:"members,omitempty"`
	// Mentors is a list of users that belong to this team that will be excluded from auto review assignments.
	Mentors []string `json:"mentors,omitempty" yaml:"mentors,omitempty"`
}

func (c *Config) IndexTeams() {
	allTeams := map[string]*TeamConfig{}
	getAllTeams(c.Teams, allTeams)
	applyTeamOverrides(c.TeamOverrides, allTeams)
	c.AllTeams = allTeams

}

func normalizeTeam(team *TeamConfig) {
	sort.Strings(team.Members)
	team.Members = slices.Compact(team.Members)
	team.Mentors = make([]string, 0)
	team.CodeReviewAssignment.ExcludedMembers = nil
	for _, child := range team.Children {
		normalizeTeam(child)
	}
}

type NormalizeOpts struct {
	Repositories bool
	Members      bool
	Teams        bool
}

// Normalize removes all content from the config that is not reflected in the
// upstream GitHub configuration.
//
// Members excluded from code review assignments are also removed due to lack
// of support to fetch this configuration in API 2022-11-28.
func (c *Config) Normalize(cfg NormalizeOpts) {
	if !cfg.Repositories {
		c.Repositories = nil
	}
	if cfg.Members {
		for name, member := range c.Members {
			member.Name = ""
			member.SlackID = ""
			c.Members[name] = member
		}
	} else {
		c.Members = nil
	}
	if cfg.Teams {
		for _, team := range c.Teams {
			normalizeTeam(team)
		}
	} else {
		c.Teams = nil
	}

	c.ExcludeCRAFromAllTeams = nil
	c.TeamOverrides = nil
	c.AllTeams = nil
	c.IndexTeams()
}

func (c *Config) Equals(other *Config) bool {
	local, err := json.Marshal(c)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not compare configurations: marshalling local config: %s", err)
	}
	remote, err := json.Marshal(other)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not compare configurations: marshalling remote config: %s", err)
	}

	return reflect.DeepEqual(local, remote)
}

func applyTeamOverrides(teams map[string]*OverrideTeamConfig, allTeams map[string]*TeamConfig) {
	for teamName, team := range teams {
		if _, ok := allTeams[teamName]; ok {
			allTeams[teamName].Members = team.Members
			allTeams[teamName].Mentors = team.Mentors
		} else {
			fmt.Fprintf(os.Stderr, "Override Warning: team %s missing from config\n", teamName)
		}
	}
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

func (c *Config) Merge(other *Config) (*Config, error) {
	// Keep the code review assignment since we can't fetch this information
	// from GitHub.
	other.ExcludeCRAFromAllTeams = c.ExcludeCRAFromAllTeams
	for i, login := range other.ExcludeCRAFromAllTeams {
		if _, ok := c.Members[login]; !ok {
			slices.Delete(other.ExcludeCRAFromAllTeams, i, i+1)
		}
	}
	// Keep mentors since we can't fetch this information
	// from GitHub.
	for otherTeamName, otherTeam := range other.AllTeams {
		team, ok := c.AllTeams[otherTeamName]
		if !ok {
			continue
		}
		otherTeam.Mentors = nil
		for _, mentor := range team.Mentors {
			for _, member := range otherTeam.Members {
				if member == mentor {
					otherTeam.Mentors = append(
						otherTeam.Mentors, mentor)
					break
				}
			}
		}
	}

	// Keep the code review assignment since we can't fetch this information
	// from GitHub.
	for otherTeamName, otherTeam := range other.AllTeams {
		team, ok := c.AllTeams[otherTeamName]
		if !ok {
			continue
		}
		if !otherTeam.CodeReviewAssignment.Enabled {
			continue
		}
		otherTeam.CodeReviewAssignment.ExcludedMembers = nil

		for _, excludedMember := range team.CodeReviewAssignment.ExcludedMembers {
			for _, member := range otherTeam.Members {
				if member == excludedMember.Login {
					otherTeam.CodeReviewAssignment.ExcludedMembers = append(
						otherTeam.CodeReviewAssignment.ExcludedMembers, excludedMember)
					break
				}
			}
		}

		// Keep the include child team members since we can't fetch this
		// information from GitHub.
		otherTeam.CodeReviewAssignment.IncludeChildTeamMembers =
			team.CodeReviewAssignment.IncludeChildTeamMembers
	}

	// Keep the reason why collaborators have been added.
	for login, oc := range c.Collaborators {
		other.Collaborators[login] = oc
	}

	// Only update the members' name if the local version is empty.
	for login, member := range c.Members {
		otherMember, ok := other.Members[login]
		if !ok {
			continue
		}
		if member.Name != "" {
			otherMember.Name = member.Name
		}
		if member.SlackID != "" {
			otherMember.SlackID = member.SlackID
		}
		other.Members[login] = otherMember
	}

	return other, nil
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

	// Mentors is a list of users that belong to this team, but opt out of code review notifications
	// Note 1: Mentors _must_ be in the member list. Lint will warn if they are not
	Mentors []string `json:"mentors,omitempty" yaml:"mentors,omitempty"`

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

	// IncludeChildTeamMembers to include the members of any child teams when
	// assigning requests. Optional.
	IncludeChildTeamMembers *bool `json:"includeChildTeamMembers,omitempty" yaml:"includeChildTeamMembers,omitempty"`
}

type TeamReviewAssignmentAlgorithm string

const (
	TeamReviewAssignmentAlgorithmLoadBalance TeamReviewAssignmentAlgorithm = "LOAD_BALANCE"
	TeamReviewAssignmentAlgorithmRoundRobin  TeamReviewAssignmentAlgorithm = "ROUND_ROBIN"
)

func RemoveExcludedMember(slice []ExcludedMember, elementToRemove string) []ExcludedMember {
	for i, element := range slice {
		if element.Login == elementToRemove {
			slice[i] = slice[len(slice)-1]
			return slice[:len(slice)-1]
		}
	}
	return slice
}
