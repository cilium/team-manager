// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package main

import (
	"fmt"

	"github.com/cilium/team-manager/pkg/config"

	gh "github.com/google/go-github/v33/github"
	"github.com/spf13/cobra"
)

var addTeamsCmd = &cobra.Command{
	Use:   "add-team TEAM [TEAM ...]",
	Short: "Add team to local configuration",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, ghClient, err := InitState()
		if err != nil {
			panic(err)
		}

		if err = addTeamsToConfig(args, cfg, ghClient); err != nil {
			panic(err)
		}
		if err = StoreState(cfg); err != nil {
			panic(err)
		}
	},
}

var setTeamsUsersCmd = &cobra.Command{
	Use:   "set-team --team TEAM USER [USER ...]",
	Short: "Set members of a team in local configuration",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, _, err := InitState()
		if err != nil {
			panic(err)
		}

		for _, t := range addTeams {
			if err = setTeamMembers(t, args, cfg); err != nil {
				panic(err)
			}
		}
		if err = StoreState(cfg); err != nil {
			panic(err)
		}
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
