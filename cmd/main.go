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
	"os"
	"os/signal"

	"github.com/spf13/cobra"
)

var (
	orgName          string
	configFilename   string
	overrideFilename string
)

func init() {
	flag := rootCmd.PersistentFlags()

	flag.StringVar(&orgName, "org", "cilium", "GitHub organization name")
	flag.StringVar(&configFilename, "config-filename", "team-assignments.yaml", "Config filename")
	flag.StringVar(&overrideFilename, "override-filename", "", "Team Override filename")
}

var rootCmd = &cobra.Command{
	Use:   "team-manager",
	Short: "Manage GitHub team state locally and synchronize it with GitHub",
}

func main() {
	ctx := interruptableContext()

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}

func interruptableContext() context.Context {
	var ctx, cancel = context.WithCancel(context.Background())

	go func() {
		signalCh := make(chan os.Signal, 1)
		signal.Notify(signalCh, os.Interrupt)
		<-signalCh
		cancel()
	}()

	return ctx
}
