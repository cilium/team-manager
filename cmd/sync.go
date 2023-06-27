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
	"github.com/cilium/team-manager/pkg/team"
)

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Fetch team assignments from GitHub",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, ghClient, err := InitState()
		ghGraphQLClient := github.NewClientGraphQL(os.Getenv("GITHUB_TOKEN"))
		tm := team.NewManager(ghClient, ghGraphQLClient, orgName)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("failed to initialize state: %w", err)
			}

			_, _ = fmt.Fprintf(os.Stderr, "Configuration file %q not found, retriving configuration from organization...\n", configFilename)
			cfg, err = tm.GetCurrentConfig(globalCtx)
			if err != nil {
				return fmt.Errorf("failed to fetch current config from GitHub: %w", err)
			}
			_, _ = fmt.Fprintf(os.Stderr, "Done, change your local configuration and re-run me again.\n")
		}
		if err = StoreState(cfg); err != nil {
			return fmt.Errorf("failed to store state to config: %w", err)
		}

		return nil
	},
}

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Update team assignments in GitHub from local files",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, ghClient, err := InitState()
		if err != nil {
			return fmt.Errorf("failed to initialize state: %w", err)
		}
		if err = config.SanityCheck(cfg); err != nil {
			return fmt.Errorf("failed to perform sanity check: %w", err)
		}
		if dryRun {
			return nil
		}

		ghGraphQLClient := github.NewClientGraphQL(os.Getenv("GITHUB_TOKEN"))
		tm := team.NewManager(ghClient, ghGraphQLClient, orgName)
		if _, err = tm.SyncTeams(globalCtx, cfg, force); err != nil {
			return fmt.Errorf("failed to sync teams to GitHub: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(pullCmd)
	rootCmd.AddCommand(pushCmd)
}
