// Copyright 2021 Authors of Cilium
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"

	gh "github.com/google/go-github/v33/github"
	"github.com/spf13/cobra"

	"github.com/cilium/team-manager/pkg/config"
	"github.com/cilium/team-manager/pkg/github"
	"github.com/cilium/team-manager/pkg/persistence"
	"github.com/cilium/team-manager/pkg/team"
)

var (
	orgName        string
	configFilename string
	force          bool
	dryRun         bool
	addUsers       []string
	addTeams       []string
	setTopHat      []string
	addPTO         []string
	removePTO      []string

	rootCmd = &cobra.Command{
		Use:   "team-manager",
		Short: "Manage GitHub team state locally and synchronize it with GitHub",
		RunE:  run,
	}

	errGithubToken = fmt.Errorf("Environment variable GITHUB_TOKEN must be set to interact with GitHub APIs.")
)

func init() {
	flag := rootCmd.PersistentFlags()

	flag.StringVar(&orgName, "org", "cilium", "GitHub organization name")
	flag.StringVar(&configFilename, "config-filename", "team-assignments.yaml", "Config filename")
	flag.BoolVar(&force, "force", false, "Force local changes into GitHub without asking for configuration")
	flag.BoolVar(&dryRun, "dry-run", false, "Dry run the steps without performing any write operation to GitHub")
	flag.StringSliceVar(&addUsers, "add-users", nil, "Adds new users to the configuration file")
	flag.StringSliceVar(&addTeams, "add-teams", nil, "Adds new teams to the configuration file")
	flag.StringSliceVar(&setTopHat, "set-top-hat", nil, "Sets the the members of the top hat team")
	flag.StringSliceVar(&addPTO, "add-pto", nil, "Add users on PTO")
	flag.StringSliceVar(&removePTO, "remove-pto", nil, "Remove users from PTO")

	go signals()
}

var globalCtx, cancel = context.WithCancel(context.Background())

func signals() {
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt)
	<-signalCh
	cancel()
}

func InitState() (localCfg *config.Config, ghClient *gh.Client, err error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" && !dryRun {
		return nil, nil, errGithubToken
	}
	ghClient = github.NewClient(token)

	localCfg, err = persistence.LoadState(configFilename)
	if err != nil {
		return nil, nil, err
	}
	return
}

func StoreState(cfg *config.Config) error {
	if err := config.SanityCheck(cfg); err != nil {
		return err
	}
	config.SortConfig(cfg)
	if err := persistence.StoreState(configFilename, cfg); err != nil {
		return err
	}
	return nil
}

func run(cmd *cobra.Command, args []string) error {
	localCfg, ghClient, err := InitState()

	var newConfig = localCfg

	ghGraphQLClient := github.NewClientGraphQL(os.Getenv("GITHUB_TOKEN"))
	tm := team.NewManager(ghClient, ghGraphQLClient, orgName)
	switch {
	case errors.Is(err, os.ErrNotExist):
		fmt.Printf("Configuration file %q not found, retriving configuration from organization...\n", configFilename)
		newConfig, err = tm.GetCurrentConfig(globalCtx)
		if err != nil {
			return fmt.Errorf("failed to read config from GitHub: %w", err)
		}
		fmt.Printf("Done, change your local configuration and re-run me again.\n")
	case err != nil:
		return fmt.Errorf("failed to initialize state: %w", err)
	case dryRun || len(addUsers) != 0 || len(addTeams) != 0 ||
		len(setTopHat) != 0 || len(addPTO) != 0 || len(removePTO) != 0:
		newConfig = localCfg

		if err = addUsersToConfig(addUsers, newConfig, ghClient); err != nil {
			return fmt.Errorf("failed to add users: %w", err)
		}

		if err = addTeamsToConfig(addUsers, newConfig, ghClient); err != nil {
			return fmt.Errorf("failed to add teams: %w", err)
		}

		if len(setTopHat) > 0 {
			if err = setTeamMembers("tophat", setTopHat, newConfig); err != nil {
				return fmt.Errorf("failed to set tophat team members: %w", err)
			}
		}

		if err = addCRAExclusionToConfig(addPTO, newConfig); err != nil {
			return fmt.Errorf("failed to add code review assignment exclusion: %w", err)
		}
		if err = removeCRAExclusionToConfig(removePTO, newConfig); err != nil {
			return fmt.Errorf("failed to remove code review assignment exclusion: %w", err)
		}
	default:
		err = config.SanityCheck(localCfg)
		if err != nil {
			return fmt.Errorf("failed to perform sanity check: %w", err)
		}
		newConfig, err = tm.SyncTeams(globalCtx, localCfg, force)
		if err != nil {
			return fmt.Errorf("failed to sync teams to GitHub: %w", err)
		}
	}
	if err = StoreState(newConfig); err != nil {
		return fmt.Errorf("failed to store state to config: %w", err)
	}

	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
