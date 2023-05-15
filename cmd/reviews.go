// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package main

import (
	"github.com/cilium/team-manager/pkg/config"
)

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
