// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package main

import (
	"github.com/cilium/team-manager/pkg/config"

	"github.com/spf13/cobra"
)

var addPTOCmd = &cobra.Command{
	Use:   "add-pto USER [USER ...]",
	Short: "Exclude user from code review assignments",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, _, err := InitState()
		if err != nil {
			panic(err)
		}

		if err = addCRAExclusionToConfig(args, cfg); err != nil {
			panic(err)
		}
		if err = StoreState(cfg); err != nil {
			panic(err)
		}
	},
}

var removePTOCmd = &cobra.Command{
	Use:   "remove-pto USER [USER ...]",
	Short: "Include user in code review assignments",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, _, err := InitState()
		if err != nil {
			panic(err)
		}

		if err := removeCRAExclusionToConfig(args, cfg); err != nil {
			panic(err)
		}
		if err = StoreState(cfg); err != nil {
			panic(err)
		}
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
