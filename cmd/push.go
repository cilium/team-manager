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
	dryRun      bool
	force       bool
	pushRepos   bool
	pushMembers bool
	pushTeams   bool
)

func init() {
	rootCmd.AddCommand(pushCmd)

	pushCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Dry run the steps without performing any write operation to GitHub")
	pushCmd.Flags().BoolVar(&force, "force", false, "Force local changes into GitHub without asking for configuration")
	pushCmd.Flags().BoolVar(&pushRepos, "repositories", true, "Push repositories permissions configuration into GitHub")
	pushCmd.Flags().BoolVar(&pushMembers, "members", true, "Push members association to the organization into GitHub")
	pushCmd.Flags().BoolVar(&pushTeams, "teams", true, "Push teams organization to the organization into GitHub")
}

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Update team assignments in GitHub from local files",
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

		newCfg, err := tm.PushConfiguration(cmd.Context(), cfg, force, dryRun, pushRepos, pushMembers, pushTeams)
		if err != nil {
			return fmt.Errorf("failed to sync teams to GitHub: %w", err)
		}

		err = persistence.StoreState(configFilename, newCfg)
		if err != nil {
			return fmt.Errorf("failed to store local state: %w", err)
		}

		return nil
	},
}
