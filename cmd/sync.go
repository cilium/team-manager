// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/cilium/team-manager/pkg/config"
	"github.com/cilium/team-manager/pkg/github"
	"github.com/cilium/team-manager/pkg/team"
	"github.com/spf13/cobra"
)

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Fetch team assignments from GitHub",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, ghClient, err := InitState()
		ghGraphQLClient := github.NewClientGraphQL(os.Getenv("GITHUB_TOKEN"))
		tm := team.NewManager(ghClient, ghGraphQLClient, orgName)
		switch {
		case errors.Is(err, os.ErrNotExist):
			fmt.Fprintf(os.Stderr, "Configuration file %q not found, retriving configuration from organization...\n", configFilename)
			cfg, err = tm.GetCurrentConfig(globalCtx)
			if err != nil {
				panic(err)
			}
			fmt.Fprintf(os.Stderr, "Done, change your local configuration and re-run me again.\n")
		case err != nil:
			panic(err)
		}
		if err = StoreState(cfg); err != nil {
			panic(err)
		}
	},
}

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Update team assignments in GitHub from local files",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, ghClient, err := InitState()
		if err != nil {
			panic(err)
		}
		if err = config.SanityCheck(cfg); err != nil {
			panic(err)
		}
		if dryRun {
			return
		}

		ghGraphQLClient := github.NewClientGraphQL(os.Getenv("GITHUB_TOKEN"))
		tm := team.NewManager(ghClient, ghGraphQLClient, orgName)
		if _, err = tm.SyncTeams(globalCtx, cfg, force); err != nil {
			panic(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(pullCmd)
	rootCmd.AddCommand(pushCmd)
}
