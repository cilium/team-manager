// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package main

import (
	"fmt"
	"strings"

	gh "github.com/google/go-github/v33/github"
	"github.com/spf13/cobra"

	"github.com/cilium/team-manager/pkg/config"
)

var addUsersCmd = &cobra.Command{
	Use:   "add-user USER [USER ...]",
	Short: "Add user to local configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, ghClient, err := InitState()
		if err != nil {
			return fmt.Errorf("failed to initialize state: %w", err)
		}

		for _, t := range addTeams {
			if _, ok := cfg.Teams[t]; !ok {
				return fmt.Errorf("unknown team %q", t)
			}
		}

		if err = addUsersToConfig(args, cfg, ghClient); err != nil {
			return fmt.Errorf("failed to add user: %w", err)
		}

		for _, t := range addTeams {
			if err = addTeamMembers(t, args, cfg); err != nil {
				return fmt.Errorf("failed to add team members to team %q: %w", t, err)
			}
		}

		if err = StoreState(cfg); err != nil {
			return fmt.Errorf("failed to store state to config: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(addUsersCmd)

	addUsersCmd.Flags().StringSliceVar(&addTeams, "teams", []string{}, "Add the users to the specified teams in the local cache")
}

func addUsersToConfig(addUsers []string, cfg *config.Config, ghClient *gh.Client) error {
	for _, addUser := range addUsers {
		u, _, err := ghClient.Users.Get(globalCtx, addUser)
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

func findUser(config *config.Config, s string) (string, error) {
	// First, try to find users by exact match of the Github username.
	if _, ok := config.Members[s]; ok {
		return s, nil
	}

	// Second, try to find githubUsernames by substring matching their name.
	var githubUsernames []string
	for githubUsername, user := range config.Members {
		if strings.Contains(strings.ToLower(user.Name), strings.ToLower(s)) {
			githubUsernames = append(githubUsernames, githubUsername)
		}
	}
	switch len(githubUsernames) {
	case 0:
		return "", fmt.Errorf("%s: user not found", s)
	case 1:
		return githubUsernames[0], nil
	default:
		return "", fmt.Errorf("%s: ambiguous user (found %s)", s, strings.Join(githubUsernames, ", "))
	}
}

func findUsers(config *config.Config, ss []string) ([]string, error) {
	users := make([]string, 0, len(ss))
	for _, s := range ss {
		user, err := findUser(config, s)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}
