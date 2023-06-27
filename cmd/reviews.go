// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cilium/team-manager/pkg/config"
)

var addPTOCmd = &cobra.Command{
	Use:   "add-pto USER [USER ...]",
	Short: "Exclude user from code review assignments",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _, err := InitState()
		if err != nil {
			return fmt.Errorf("failed to initialize state: %w", err)
		}

		if err = addCRAExclusionToConfig(args, cfg); err != nil {
			return fmt.Errorf("failed to add code review assignment exclusion: %w", err)
		}
		if err = StoreState(cfg); err != nil {
			return fmt.Errorf("failed to store state to config: %w", err)
		}

		return nil
	},
}

var removePTOCmd = &cobra.Command{
	Use:   "remove-pto USER [USER ...]",
	Short: "Include user in code review assignments",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _, err := InitState()
		if err != nil {
			return fmt.Errorf("failed to initialize state: %w", err)
		}

		if err := removeCRAExclusionToConfig(args, cfg); err != nil {
			return fmt.Errorf("failed to remove code review assignment exclusion: %w", err)
		}
		if err = StoreState(cfg); err != nil {
			return fmt.Errorf("failed to store state to config: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(addPTOCmd)
	rootCmd.AddCommand(removePTOCmd)
}

func addCRAExclusionToConfig(addCRAExclusion []string, cfg *config.Config) error {
	excludeCRAFromAllTeams := newStringSet(cfg.ExcludeCRAFromAllTeams...)
	for _, s := range addCRAExclusion {
		user, err := findUser(cfg, s)
		if err != nil {
			return err
		}
		excludeCRAFromAllTeams.add(user)
	}
	cfg.ExcludeCRAFromAllTeams = excludeCRAFromAllTeams.elements()

	return nil
}

func removeCRAExclusionToConfig(addCRAExclusion []string, cfg *config.Config) error {
	excludeCRAFromAllTeams := newStringSet(cfg.ExcludeCRAFromAllTeams...)
	for _, s := range removePTO {
		user, err := findUser(cfg, s)
		if err != nil {
			return err
		}
		excludeCRAFromAllTeams.remove(user)
	}
	cfg.ExcludeCRAFromAllTeams = excludeCRAFromAllTeams.elements()

	return nil
}
