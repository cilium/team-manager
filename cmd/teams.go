// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package main

import (
	"fmt"

	gh "github.com/google/go-github/v33/github"
	"github.com/spf13/cobra"

	"github.com/cilium/team-manager/pkg/config"
)

var addTeamsCmd = &cobra.Command{
	Use:   "add-team TEAM [TEAM ...]",
	Short: "Add team to local configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, ghClient, err := InitState()
		if err != nil {
			return fmt.Errorf("failed to initialize state: %w", err)
		}

		if err = addTeamsToConfig(args, cfg, ghClient); err != nil {
			return fmt.Errorf("failed to add teams to config: %w", err)
		}
		if err = StoreState(cfg); err != nil {
			return fmt.Errorf("failed to store state to config: %w", err)
		}

		return nil
	},
}

var setTeamsUsersCmd = &cobra.Command{
	Use:   "set-team --team TEAM USER [USER ...]",
	Short: "Set members of a team in local configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _, err := InitState()
		if err != nil {
			return fmt.Errorf("failed to initialize state: %w", err)
		}

		for _, t := range addTeams {
			if err = setTeamMembers(t, args, cfg); err != nil {
				return fmt.Errorf("failed to set team members: %w", err)
			}
		}
		if err = StoreState(cfg); err != nil {
			return fmt.Errorf("failed to store state to config: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(addTeamsCmd)
	rootCmd.AddCommand(setTeamsUsersCmd)

	setTeamsUsersCmd.Flags().StringSliceVar(&addTeams, "teams", []string{}, "Team whose membership should be modified locally")
}

func addTeamsToConfig(addTeams []string, cfg *config.Config, ghClient *gh.Client) error {
	for _, addTeam := range addTeams {
		u, _, err := ghClient.Users.Get(globalCtx, addTeam)
		if err != nil {
			return err
		}
		cfg.Members[u.GetLogin()] = config.User{
			ID:   u.GetNodeID(),
			Name: u.GetName(),
		}
	}

	return nil
}

func setTeamMembers(team string, users []string, cfg *config.Config) error {
	members, err := findUsers(cfg, users)
	if err != nil {
		return fmt.Errorf("unable to find users: %w", err)
	}
	teamConfig, ok := cfg.Teams[team]
	if !ok {
		return fmt.Errorf("unknown team %q", team)
	}
	teamConfig.Members = newStringSet(members...).elements()
	cfg.Teams[team] = teamConfig

	return nil
}

func addTeamMembers(team string, users []string, cfg *config.Config) error {
	teamConfig, ok := cfg.Teams[team]
	if !ok {
		return fmt.Errorf("unknown team %q", team)
	}
	newMembers := newStringSet(append(teamConfig.Members, users...)...)
	return setTeamMembers(team, newMembers.elements(), cfg)
}
