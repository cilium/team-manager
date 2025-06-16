// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/cilium/team-manager/pkg/config"
	"github.com/cilium/team-manager/pkg/github"
	"github.com/cilium/team-manager/pkg/persistence"
	"github.com/cilium/team-manager/pkg/team"
)

var (
	opts config.NormalizeOpts
)

func init() {
	rootCmd.AddCommand(diffCmd)

	diffCmd.Flags().BoolVar(&opts.Repositories, "repositories", true, "Compare repositories permissions configuration in GitHub")
	diffCmd.Flags().BoolVar(&opts.Members, "members", true, "Compare members association to the organization in GitHub")
	diffCmd.Flags().BoolVar(&opts.Teams, "teams", true, "Compare teams organization to the organization in GitHub")
}

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Display a diff between the local and remote configuration",
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := persistence.LoadState(configFilename, overrideFilename)
		if err != nil {
			return fmt.Errorf("failed to load local state: %w", err)
		}

		if err = config.SanityCheck(cfg); err != nil {
			return fmt.Errorf("failed to perform sanity check: %w", err)
		}
		config.SortConfig(cfg)
		cfg.Normalize(opts)

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

		diff, err := tm.Diff(cmd.Context(), cfg, opts)
		if err != nil {
			return fmt.Errorf("failed to sync teams to GitHub: %w", err)
		}

		if diff != "" {
			fmt.Printf("%s", diff)
			os.Exit(1)
		}

		return nil
	},
}
