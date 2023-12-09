// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package main

import (
	"context"
	"fmt"

	gh "github.com/google/go-github/v57/github"
	"github.com/spf13/cobra"

	"github.com/cilium/team-manager/pkg/config"
	"github.com/cilium/team-manager/pkg/github"
	"github.com/cilium/team-manager/pkg/persistence"
	"github.com/cilium/team-manager/pkg/stringset"
)

func init() {
	rootCmd.AddCommand(addTeamsCmd)
	rootCmd.AddCommand(setTeamsUsersCmd)
}

var addTeamsCmd = &cobra.Command{
	Use:   "add-team TEAM [TEAM ...]",
	Short: "Add team to local configuration by their slug name",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ghClient, err := github.NewClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create github client: %w", err)
		}

		cfg, err := persistence.LoadState(configFilename)
		if err != nil {
			return fmt.Errorf("failed to load local state: %w", err)
		}

		if err = addTeamsToConfig(cmd.Context(), args, cfg, ghClient); err != nil {
			return fmt.Errorf("failed to add teams to config: %w", err)
		}
		if err = persistence.StoreState(configFilename, cfg); err != nil {
			return fmt.Errorf("failed to store state to config: %w", err)
		}

		return nil
	},
}

var setTeamsUsersCmd = &cobra.Command{
	Use:   "set-team TEAM USER [USER ...]",
	Short: "Set members of a team in local configuration",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := persistence.LoadState(configFilename)
		if err != nil {
			return fmt.Errorf("failed to load local state: %w", err)
		}

		if err = setTeamMembers(args[0], args[1:], cfg); err != nil {
			return fmt.Errorf("failed to set team members: %w", err)
		}

		if err = persistence.StoreState(configFilename, cfg); err != nil {
			return fmt.Errorf("failed to store state to config: %w", err)
		}

		return nil
	},
}

func addTeamsToConfig(ctx context.Context, addTeams []string, cfg *config.Config, ghClient *gh.Client) error {
	for _, addTeam := range addTeams {
		t, _, err := ghClient.Teams.GetTeamBySlug(ctx, orgName, addTeam)
		if err != nil {
			return fmt.Errorf("failed to get GitHub team: %w", err)
		}
		if _, ok := cfg.AllTeams[t.GetName()]; ok {
			return fmt.Errorf("team %q already exists", t.GetName())
		}
		team := &config.TeamConfig{
			ID:          t.GetNodeID(),
			RESTID:      t.GetID(),
			Description: t.GetDescription(),
			Privacy:     config.ParsePrivacyFromREST(t.GetPrivacy()),
		}
		if t.GetParent() != nil {
			team.ParentTeam = config.TeamOrMemberName(t.GetParent().GetName())
		}

		page := 0
		for {
			members, resp, err := ghClient.Teams.ListTeamMembersBySlug(ctx, orgName, addTeam, &gh.TeamListTeamMembersOptions{
				ListOptions: gh.ListOptions{
					Page:    page,
					PerPage: 100,
				},
			})
			if err != nil {
				return fmt.Errorf("failed to get members of team: %w", err)
			}
			for _, member := range members {
				team.Members = append(team.Members, member.GetLogin())
			}
			if resp.NextPage == 0 {
				break
			}
			page = resp.NextPage
		}
		cfg.Teams[t.GetName()] = team
		cfg.AllTeams[t.GetName()] = team
	}

	return nil
}

func setTeamMembers(team string, users []string, cfg *config.Config) error {
	members, err := findUsers(cfg, users)
	if err != nil {
		return fmt.Errorf("unable to find users: %w", err)
	}
	teamConfig, ok := cfg.AllTeams[team]
	if !ok {
		return fmt.Errorf("unknown team %q", team)
	}
	teamConfig.Members = stringset.New(members...).Elements()
	cfg.AllTeams[team] = teamConfig

	return nil
}

func addTeamMembers(team string, users []string, cfg *config.Config) error {
	teamConfig, ok := cfg.AllTeams[team]
	if !ok {
		return fmt.Errorf("unknown team %q", team)
	}
	newMembers := stringset.New(append(teamConfig.Members, users...)...)
	return setTeamMembers(team, newMembers.Elements(), cfg)
}
