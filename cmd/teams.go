// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package main

import (
	"fmt"

	"github.com/cilium/team-manager/pkg/config"

	gh "github.com/google/go-github/v33/github"
)

func addTeamsToConfig(addTeams []string, cfg *config.Config, ghClient *gh.Client) error {
	for _, addTeam := range addTeams {
		u, _, err := ghClient.Users.Get(globalCtx, addTeam)
		if err != nil {
			return err
		}
		cfg.Members[u.GetLogin()] = config.User{
			ID:   u.GetNodeID(),
			Name: u.GetName(),
		}
	}

	return nil
}

func setTeamMembers(team string, users []string, cfg *config.Config) error {
	members, err := findUsers(cfg, users)
	if err != nil {
		return fmt.Errorf("unable to find users: %w", err)
	}
	teamConfig, ok := cfg.Teams[team]
	if !ok {
		return fmt.Errorf("unknown team %q", team)
	}
	teamConfig.Members = newStringSet(members...).elements()
	cfg.Teams[team] = teamConfig

	return nil
}
