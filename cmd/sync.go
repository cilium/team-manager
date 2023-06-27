// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/cilium/team-manager/pkg/config"
	"github.com/cilium/team-manager/pkg/github"
	"github.com/cilium/team-manager/pkg/persistence"
	"github.com/cilium/team-manager/pkg/team"
)

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Fetch team assignments from GitHub",
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, _ []string) error {
		ghClient, err := github.NewClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create github client: %w", err)
		}

		ghGraphQLClient, err := github.NewClientGraphQLFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create github graphql client: %w", err)
		}

		tm := team.NewManager(ghClient, ghGraphQLClient, orgName)

		localCfg, err := persistence.LoadState(configFilename)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("failed to load local state: %w", err)
			}

			fmt.Printf("Configuration file %q not found, retriving configuration from organization...\n", configFilename)
			localCfg, err = tm.GetCurrentConfig(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to read config from GitHub: %w", err)
			}
			fmt.Printf("Done, change your local configuration and re-run me again.\n")
		}

		if err = persistence.StoreState(configFilename, localCfg); err != nil {
			return fmt.Errorf("failed to store state to config: %w", err)
		}

		return nil
	},
}

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Update team assignments in GitHub from local files",
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := persistence.LoadState(configFilename)
		if err != nil {
			return fmt.Errorf("failed to load local state: %w", err)
		}

		if err = config.SanityCheck(cfg); err != nil {
			return fmt.Errorf("failed to perform sanity check: %w", err)
		}

		if dryRun {
			return nil
		}

		ghClient, err := github.NewClientFromEnv()
		if err != nil && !dryRun {
			return fmt.Errorf("failed to create github client: %w", err)
		}

		ghGraphQLClient, err := github.NewClientGraphQLFromEnv()
		if err != nil && !dryRun {
			return fmt.Errorf("failed to create github graphql client: %w", err)
		}
		tm := team.NewManager(ghClient, ghGraphQLClient, orgName)

		if _, err = tm.SyncTeams(cmd.Context(), cfg, force); err != nil {
			return fmt.Errorf("failed to sync teams to GitHub: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(pullCmd)
	rootCmd.AddCommand(pushCmd)
}
