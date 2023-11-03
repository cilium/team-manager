// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package main

import (
	"fmt"
	"github.com/spf13/cobra"

	"github.com/cilium/team-manager/pkg/config"
	"github.com/cilium/team-manager/pkg/github"
	"github.com/cilium/team-manager/pkg/persistence"
	"github.com/cilium/team-manager/pkg/team"
)

var (
	dryRun bool
	force  bool
)

func init() {
	rootCmd.AddCommand(pushCmd)

	pushCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Dry run the steps without performing any write operation to GitHub")
	pushCmd.Flags().BoolVar(&force, "force", false, "Force local changes into GitHub without asking for configuration")
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

		ghClient, err := github.NewClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create github client: %w", err)
		}

		ghGraphQLClient, err := github.NewClientGraphQLFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create github graphql client: %w", err)
		}
		tm, err := team.NewManager(ghClient, ghGraphQLClient, orgName)
		if err != nil {
			return fmt.Errorf("unable to initialize manager %w", err)
		}

		if _, err = tm.SyncTeams(cmd.Context(), cfg, force, dryRun); err != nil {
			return fmt.Errorf("failed to sync teams to GitHub: %w", err)
		}

		return nil
	},
}
