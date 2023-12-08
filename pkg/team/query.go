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

package team

import (
	"context"

	"github.com/shurcooL/githubv4"
)

func (tm *Manager) queryOrgRepos(ctx context.Context, additionalVariables map[string]interface{}) (queryResultRepositories, error) {
	var q queryResultRepositories
	variables := map[string]interface{}{
		"repositoryOwner":            githubv4.String(tm.owner),
		"repositoriesWithRoleCursor": (*githubv4.String)(nil), // Null after argument to get first page.
		"collaboratorAffiliation":    (*githubv4.CollaboratorAffiliation)(nil),
		"collaboratorsCursor":        (*githubv4.String)(nil), // Null after argument to get first page.
	}

	for k, v := range additionalVariables {
		variables[k] = v
	}

	err := tm.gqlGHClient.Query(ctx, &q, variables)
	if err != nil {
		return queryResultRepositories{}, err
	}

	return q, nil
}

func (tm *Manager) queryOrgMembers(ctx context.Context, additionalVariables map[string]interface{}) (queryResultMembers, error) {
	var q queryResultMembers
	variables := map[string]interface{}{
		"repositoryOwner":       githubv4.String(tm.owner),
		"membersWithRoleCursor": (*githubv4.String)(nil), // Null after argument to get first page.
	}

	for k, v := range additionalVariables {
		variables[k] = v
	}

	err := tm.gqlGHClient.Query(ctx, &q, variables)
	if err != nil {
		return queryResultMembers{}, err
	}

	return q, nil
}

func (tm *Manager) queryTeamsRepositories(ctx context.Context, additionalVariables map[string]interface{}) (queryTeamsRepositoriesResult, error) {
	var q queryTeamsRepositoriesResult
	variables := map[string]interface{}{
		"repositoryOwner":    githubv4.String(tm.owner),
		"teamsCursor":        (*githubv4.String)(nil), // Null after argument to get first page.
		"repositoriesCursor": (*githubv4.String)(nil), // Null after argument to get first page.
	}

	for k, v := range additionalVariables {
		variables[k] = v
	}

	err := tm.gqlGHClient.Query(ctx, &q, variables)
	if err != nil {
		return queryTeamsRepositoriesResult{}, err
	}

	return q, nil
}

func (tm *Manager) queryTeamsMembers(ctx context.Context, additionalVariables map[string]interface{}) (queryTeamsMembersResult, error) {
	var q queryTeamsMembersResult
	variables := map[string]interface{}{
		"repositoryOwner": githubv4.String(tm.owner),
		"teamsCursor":     (*githubv4.String)(nil), // Null after argument to get first page.
		"membersCursor":   (*githubv4.String)(nil), // Null after argument to get first page.
	}

	for k, v := range additionalVariables {
		variables[k] = v
	}

	err := tm.gqlGHClient.Query(ctx, &q, variables)
	if err != nil {
		return queryTeamsMembersResult{}, err
	}

	return q, nil
}

func (tm *Manager) queryOrgMemberLimitedAvailability(ctx context.Context, additionalVariables map[string]interface{}) (queryResultMemberLimitedAvailability, error) {
	var q queryResultMemberLimitedAvailability
	variables := map[string]interface{}{
		"login": (githubv4.String)(""),
	}

	for k, v := range additionalVariables {
		variables[k] = v
	}

	err := tm.gqlGHClient.Query(ctx, &q, variables)
	if err != nil {
		return queryResultMemberLimitedAvailability{}, err
	}

	return q, nil
}
