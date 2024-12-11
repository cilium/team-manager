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

func init() {
	rootCmd.AddCommand(syncCmd)
}

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Merges the configuration from GitHub into the local configuration",
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := persistence.LoadState(configFilename, overrideFilename)
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

		if (orgName != "" && orgName != cfg.Organization) ||
			(cfg.Organization != "" && orgName != cfg.Organization) {
			return fmt.Errorf("Organization name different than the one in the configfile. %q != %q\n", orgName, cfg.Organization)
		}

		tm, err := team.NewManager(ghClient, ghGraphQLClient, orgName)
		if err != nil {
			return fmt.Errorf("unable to initialize manager %w", err)
		}

		newCfg, err := tm.PullConfiguration(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to sync teams to GitHub: %w", err)
		}

		mergedCfg, err := cfg.Merge(newCfg)
		if err != nil {
			return fmt.Errorf("unable to merge upstream configuration to local config: %w", err)
		}

		err = persistence.StoreState(configFilename, mergedCfg)
		if err != nil {
			return fmt.Errorf("failed to store local state: %w", err)
		}

		return nil
	},
}
