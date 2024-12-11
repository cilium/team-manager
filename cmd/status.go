// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package main

import (
	"context"
	"fmt"

	"github.com/cilium/team-manager/pkg/github"
	"github.com/cilium/team-manager/pkg/team"
	"github.com/spf13/cobra"

	"github.com/cilium/team-manager/pkg/config"
	"github.com/cilium/team-manager/pkg/persistence"
)

func init() {
	rootCmd.AddCommand(statusCmd)
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Checks user status for all teams",
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, _ []string) error {

		localCfg, err := persistence.LoadState(configFilename, overrideFilename)
		if err != nil {
			return fmt.Errorf("failed to load local state: %w", err)
		}

		err = config.SanityCheck(localCfg)
		if err != nil {
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

		err = tm.CheckUserStatus(context.Background(), localCfg)
		if err != nil {
			return err
		}

		return nil
	},
}
