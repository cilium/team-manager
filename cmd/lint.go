// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cilium/team-manager/pkg/config"
	"github.com/cilium/team-manager/pkg/persistence"
)

var lintCmd = &cobra.Command{
	Use:   "lint",
	Short: "Checks and formats local config",
	RunE: func(cmd *cobra.Command, args []string) error {

		localCfg, err := persistence.LoadState(configFilename)
		if err != nil {
			return fmt.Errorf("failed to load local state: %w", err)
		}

		err = config.SanityCheck(localCfg)
		if err != nil {
			return fmt.Errorf("failed to perform sanity check: %w", err)
		}

		if err = persistence.StoreState(configFilename, localCfg); err != nil {
			return fmt.Errorf("failed to store state to config: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(lintCmd)
}
