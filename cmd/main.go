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
	"strings"

	"github.com/cilium/team-manager/pkg/config"
	"github.com/cilium/team-manager/pkg/github"
	"github.com/cilium/team-manager/pkg/persistence"
	"github.com/cilium/team-manager/pkg/team"

	flag "github.com/spf13/pflag"
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
)

func init() {
	flag.StringVar(&orgName, "org", "cilium", "GitHub organization name")
	flag.StringVar(&configFilename, "config-filename", "team-assignments.yaml", "Config filename")
	flag.BoolVar(&force, "force", false, "Force local changes into GitHub without asking for configuration")
	flag.BoolVar(&dryRun, "dry-run", false, "Dry run the steps without performing any write operation to GitHub")
	flag.StringSliceVar(&addUsers, "add-users", nil, "Adds new users to the configuration file")
	flag.StringSliceVar(&addTeams, "add-teams", nil, "Adds new teams to the configuration file")
	flag.StringSliceVar(&setTopHat, "set-top-hat", nil, "Sets the the members of the top hat team")
	flag.StringSliceVar(&addPTO, "add-pto", nil, "Add users on PTO")
	flag.StringSliceVar(&removePTO, "remove-pto", nil, "Remove users from PTO")
	flag.Parse()

	go signals()
}

var globalCtx, cancel = context.WithCancel(context.Background())

func signals() {
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt)
	<-signalCh
	cancel()
}

func main() {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		panic("GITHUB_TOKEN must be set to interact with GitHub APIs.")
	}
	ghClient := github.NewClient(token)
	ghGraphQLClient := github.NewClientGraphQL(token)

	tm := team.NewManager(ghClient, ghGraphQLClient, orgName)

	var newConfig *config.Config
	localCfg, err := persistence.LoadState(configFilename)

	switch {
	case errors.Is(err, os.ErrNotExist):
		fmt.Printf("Configuration file %q not found, retriving configuration from organization...\n", configFilename)
		newConfig, err = tm.GetCurrentConfig(globalCtx)
		if err != nil {
			panic(err)
		}
		fmt.Printf("Done, change your local configuration and re-run me again.\n")
	case err != nil:
		panic(err)
	case dryRun || len(addUsers) != 0 || len(addTeams) != 0 ||
		len(setTopHat) != 0 || len(addPTO) != 0 || len(removePTO) != 0:
		newConfig = localCfg

		for _, addUser := range addUsers {
			u, _, err := ghClient.Users.Get(globalCtx, addUser)
			if err != nil {
				panic(err)
			}
			newConfig.Members[u.GetLogin()] = config.User{
				ID:   u.GetNodeID(),
				Name: u.GetName(),
			}
		}

		for _, addTeam := range addTeams {
			t, _, err := ghClient.Teams.GetTeamBySlug(globalCtx, orgName, addTeam)
			if err != nil {
				panic(err)
			}
			newConfig.Teams[t.GetName()] = config.TeamConfig{
				ID: t.GetNodeID(),
			}
		}

		if len(setTopHat) > 0 {
			members, err := findUsers(newConfig, setTopHat)
			if err != nil {
				panic(err)
			}
			teamConfig, ok := newConfig.Teams["tophat"]
			if !ok {
				panic("unknown team tophat")
			}
			teamConfig.Members = newStringSet(members...).elements()
			newConfig.Teams["tophat"] = teamConfig
		}

		excludeCRAFromAllTeams := newStringSet(newConfig.ExcludeCRAFromAllTeams...)
		for _, s := range addPTO {
			user, err := findUser(newConfig, s)
			if err != nil {
				panic(err)
			}
			excludeCRAFromAllTeams.add(user)
		}
		for _, s := range removePTO {
			user, err := findUser(newConfig, s)
			if err != nil {
				panic(err)
			}
			excludeCRAFromAllTeams.remove(user)
		}
		newConfig.ExcludeCRAFromAllTeams = excludeCRAFromAllTeams.elements()

		err = config.SanityCheck(localCfg)
		if err != nil {
			panic(err)
		}
	default:
		err = config.SanityCheck(localCfg)
		if err != nil {
			panic(err)
		}
		newConfig, err = tm.SyncTeams(globalCtx, localCfg, force)
		if err != nil {
			panic(err)
		}
	}

	config.SortConfig(newConfig)

	err = persistence.StoreState(configFilename, newConfig)
	if err != nil {
		panic(err)
	}
}

func findUser(config *config.Config, s string) (string, error) {
	// First, try to find users by exact match of the Github username.
	if _, ok := config.Members[s]; ok {
		return s, nil
	}

	// Second, try to find githubUsernames by substring matching their name.
	var githubUsernames []string
	for githubUsername, user := range config.Members {
		if strings.Contains(strings.ToLower(user.Name), strings.ToLower(s)) {
			githubUsernames = append(githubUsernames, githubUsername)
		}
	}
	switch len(githubUsernames) {
	case 0:
		return "", fmt.Errorf("%s: user not found", s)
	case 1:
		return githubUsernames[0], nil
	default:
		return "", fmt.Errorf("%s: ambiguous user (found %s)", s, strings.Join(githubUsernames, ", "))
	}
}

func findUsers(config *config.Config, ss []string) ([]string, error) {
	users := make([]string, 0, len(ss))
	for _, s := range ss {
		user, err := findUser(config, s)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}
