// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/cilium/team-manager/pkg/github"
	"github.com/cilium/team-manager/pkg/persistence"
	"github.com/cilium/team-manager/pkg/team"
)

func init() {
	rootCmd.AddCommand(initCmd)
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initializing the config file by fetching team assignments from GitHub",
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

		if _, err := persistence.LoadState(configFilename); err == nil {
			fmt.Printf("Configuration file %q already exists\n", configFilename)
			return nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("failed to load local state: %w", err)
		}

		fmt.Println("Retrieving configuration from organization...")
		remoteCfg, err := tm.GetCurrentConfig(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to read config from GitHub: %w", err)
		}

		fmt.Printf("Creating configuration file %q...\n", configFilename)
		if err = persistence.StoreState(configFilename, remoteCfg); err != nil {
			return fmt.Errorf("failed to store state to config: %w", err)
		}

		return nil
	},
}
