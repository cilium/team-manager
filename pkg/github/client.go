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

package github

import (
	"context"
	"fmt"
	"os"

	gh "github.com/google/go-github/v67/github"
	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
)

var errGithubToken = fmt.Errorf("environment variable GITHUB_TOKEN must be set to interact with GitHub APIs")

func NewClientFromEnv() (*gh.Client, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, errGithubToken
	}

	return NewClient(token), nil
}

func NewClient(ghToken string) *gh.Client {
	return gh.NewClient(
		oauth2.NewClient(
			context.Background(),
			oauth2.StaticTokenSource(
				&oauth2.Token{
					AccessToken: ghToken,
				},
			),
		),
	)
}

func NewClientGraphQLFromEnv() (*githubv4.Client, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, errGithubToken
	}

	return NewClientGraphQL(token), nil
}

func NewClientGraphQL(ghToken string) *githubv4.Client {
	return githubv4.NewClientWithAcceptHeaders(
		oauth2.NewClient(
			context.Background(),
			oauth2.StaticTokenSource(
				&oauth2.Token{
					AccessToken: ghToken,
				},
			),
		),
		[]string{
			// Set header for team review assignments preview: https://docs.github.com/en/graphql/overview/schema-previews#team-review-assignments-preview
			"application/vnd.github.stone-crop-preview+json",
		},
	)
}
