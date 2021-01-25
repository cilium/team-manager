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
)

func init() {
	flag.StringVar(&orgName, "org", "cilium", "GitHub organization name")
	flag.StringVar(&configFilename, "config-filename", "team-assignments.yaml", "GitHub organization and repository names separated by a slash")
	flag.BoolVar(&force, "force", false, "Force local changes into GitHub without asking for configuration")
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
	ghClient := github.NewClient(os.Getenv("GITHUB_TOKEN"))
	ghGraphQLClient := github.NewClientGraphQL(os.Getenv("GITHUB_TOKEN"))

	tm := team.NewManager(ghClient, ghGraphQLClient, orgName)

	var newConfig *config.Config
	localCfg, err := persistence.LoadState(configFilename)
	if errors.Is(err, os.ErrNotExist) {
		fmt.Printf("Configuration file %q not found, retriving configuration from organization...\n", configFilename)
		newConfig, err = tm.GetCurrentConfig(globalCtx)
		if err != nil {
			panic(err)
		}
		fmt.Printf("Done, change your local configuration and re-run me again.\n")
	} else if err != nil {
		panic(err)
	} else {
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
