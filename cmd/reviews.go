// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cilium/team-manager/pkg/config"
	"github.com/cilium/team-manager/pkg/persistence"
	"github.com/cilium/team-manager/pkg/stringset"
)

func init() {
	rootCmd.AddCommand(addPTOCmd)
	rootCmd.AddCommand(removePTOCmd)
}

var addPTOCmd = &cobra.Command{
	Use:   "add-pto USER [USER ...]",
	Short: "Exclude user from code review assignments",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := persistence.LoadState(configFilename, overrideFilename)
		if err != nil {
			return fmt.Errorf("failed to load local state: %w", err)
		}

		if err = addCRAExclusionToConfig(args, cfg); err != nil {
			return fmt.Errorf("failed to add code review assignment exclusion: %w", err)
		}
		if err = persistence.StoreState(configFilename, cfg); err != nil {
			return fmt.Errorf("failed to store state to config: %w", err)
		}

		return nil
	},
}

var removePTOCmd = &cobra.Command{
	Use:   "remove-pto USER [USER ...]",
	Short: "Include user in code review assignments",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := persistence.LoadState(configFilename, overrideFilename)
		if err != nil {
			return fmt.Errorf("failed to load local state: %w", err)
		}

		if err := removeCRAExclusionToConfig(args, cfg); err != nil {
			return fmt.Errorf("failed to remove code review assignment exclusion: %w", err)
		}
		if err = persistence.StoreState(configFilename, cfg); err != nil {
			return fmt.Errorf("failed to store state to config: %w", err)
		}

		return nil
	},
}

func addCRAExclusionToConfig(addCRAExclusion []string, cfg *config.Config) error {
	excludeCRAFromAllTeams := stringset.New(cfg.ExcludeCRAFromAllTeams...)
	for _, s := range addCRAExclusion {
		user, err := findUser(cfg, s)
		if err != nil {
			return err
		}
		excludeCRAFromAllTeams.Add(user)
	}
	cfg.ExcludeCRAFromAllTeams = excludeCRAFromAllTeams.Elements()

	return nil
}

func removeCRAExclusionToConfig(addCRAExclusion []string, cfg *config.Config) error {
	excludeCRAFromAllTeams := stringset.New(cfg.ExcludeCRAFromAllTeams...)
	for _, s := range addCRAExclusion {
		user, err := findUser(cfg, s)
		if err != nil {
			return err
		}
		excludeCRAFromAllTeams.Remove(user)
	}
	cfg.ExcludeCRAFromAllTeams = excludeCRAFromAllTeams.Elements()

	return nil
}
