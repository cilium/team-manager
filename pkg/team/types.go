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
	"fmt"

	"github.com/shurcooL/githubv4"
)

// These queries can be easily generated from https://docs.github.com/en/graphql/overview/explorer

type collaboratorAffiliationEdge struct {
	Permission *githubv4.RepositoryPermission
	Node       struct {
		Login githubv4.String
	}
}

type repository struct {
	Name          githubv4.String
	Collaborators struct {
		Edges    []collaboratorAffiliationEdge
		PageInfo struct {
			EndCursor   githubv4.String
			HasNextPage githubv4.Boolean
		}
	} `graphql:"collaborators(first: 50, after: $collaboratorsCursor, affiliation: $collaboratorAffiliation)"`
}

type RepositoryGraphQL struct {
	TotalCount githubv4.Int
	Nodes      []repository
	PageInfo   struct {
		EndCursor   githubv4.String
		HasNextPage githubv4.Boolean
	}
}

func (r *RepositoryGraphQL) WithName(name githubv4.String) (repository, error) {
	for _, n := range r.Nodes {
		if n.Name == name {
			return n, nil
		}
	}
	return repository{}, fmt.Errorf("repository with name %q not found", name)
}

// queryResultRepositories was derived from
//
//	query organization {
//	  organization(login: "$repositoryOwner") {
//	    repositories(first: 50, after: $repositoriesWithRoleCursor) {
//	      totalCount
//	      pageInfo {
//	        endCursor
//	        hasNextPage
//	      }
//	      nodes {
//	        name
//	        collaborators(first: 50, affiliation: DIRECT) {
//	          edges {
//	            permission
//	            node {
//	              login
//	            }
//	          }
//	        }
//	      }
//	    }
//	  }
//	}
type queryResultRepositories struct {
	Organization struct {
		Repositories RepositoryGraphQL `graphql:"repositories(first: 50, after: $repositoriesWithRoleCursor)"`
	} `graphql:"organization(login: $repositoryOwner)"`
}

type teamMember struct {
	ID    githubv4.ID
	Login githubv4.String
	Name  githubv4.String
}

// queryResultMembers was derived from
//
//	query organization {
//	  organization(login: "$repositoryOwner") {
//	    membersWithRole(first: 50, after: $membersWithRoleCursor) {
//	      totalCount
//	      pageInfo {
//	        endCursor
//	        hasNextPage
//	      }
//	      nodes {
//	        login
//	        id
//	        name
//	      }
//	    }
//	  }
//	}
type queryResultMembers struct {
	Organization struct {
		Members struct {
			TotalCount githubv4.Int
			Nodes      []teamMember
			PageInfo   struct {
				EndCursor   githubv4.String
				HasNextPage githubv4.Boolean
			}
		} `graphql:"membersWithRole(first: 50, after: $membersWithRoleCursor)"`
	} `graphql:"organization(login: $repositoryOwner)"`
}

type teamRepositoryEdge struct {
	Permission *githubv4.RepositoryPermission
	Node       struct {
		Name githubv4.String
	}
}

type teamCommon struct {
	ID   githubv4.ID
	Name githubv4.String
}

type teamRepositories struct {
	teamCommon
	Repositories struct {
		Edges    []teamRepositoryEdge
		PageInfo struct {
			EndCursor   githubv4.String
			HasNextPage githubv4.Boolean
		}
	} `graphql:"repositories(first: 100, after: $repositoriesCursor)"`
}

type teamMembers struct {
	teamCommon
	Description githubv4.String
	DatabaseId  int64
	Privacy     githubv4.TeamPrivacy
	ParentTeam  struct {
		Name githubv4.String
	}
	ReviewRequestDelegationEnabled     githubv4.Boolean
	ReviewRequestDelegationAlgorithm   githubv4.String
	ReviewRequestDelegationMemberCount githubv4.Int
	ReviewRequestDelegationNotifyTeam  githubv4.Boolean
	Members                            struct {
		Nodes []struct {
			Login githubv4.String
		}
		PageInfo struct {
			EndCursor   githubv4.String
			HasNextPage githubv4.Boolean
		}
	} `graphql:"members(first: 30, after: $membersCursor, membership: IMMEDIATE)"`
}

type teamRepositoriesGraphQL struct {
	TotalCount githubv4.Int
	Nodes      []teamRepositories
	PageInfo   struct {
		EndCursor   githubv4.String
		HasNextPage githubv4.Boolean
	}
}

func (t *teamRepositoriesGraphQL) WithID(id githubv4.ID) (teamRepositories, error) {
	for _, n := range t.Nodes {
		if n.ID == id {
			return n, nil
		}
	}

	return teamRepositories{}, fmt.Errorf("team with id %q not found", id)
}

// queryTeamsRepositoriesResult was derived from
//
//		query organization {
//		  organization(login: "$repositoryOwner") {
//		    teams(first: 2, after: $teamsCursor) {
//	       totalCount
//		      nodes {
//		        repositories(first: 100) {
//		          edges {
//		            permission
//		            node {
//		              name
//		            }
//		          }
//		        }
//		        name
//		        privacy
//		        description
//		        parentTeam {
//		          name
//		        }
//		        id
//		      }
//		    }
//		  }
//		}
type queryTeamsRepositoriesResult struct {
	Organization struct {
		Teams teamRepositoriesGraphQL `graphql:"teams(first: 2, after: $teamsCursor)"`
	} `graphql:"organization(login: $repositoryOwner)"`
}

func (t *teamMembersGraphQL) WithID(id githubv4.ID) (teamMembers, error) {
	for _, n := range t.Nodes {
		if n.ID == id {
			return n, nil
		}
	}

	return teamMembers{}, fmt.Errorf("team with id %q not found", id)
}

type teamMembersGraphQL struct {
	Nodes    []teamMembers
	PageInfo struct {
		EndCursor   githubv4.String
		HasNextPage githubv4.Boolean
	}
}

// queryTeamsMembersResult was derived from
//
//	query organization {
//	  organization(login: "$repositoryOwner") {
//	    teams(first: 50, after: $teamsCursor) {
//	      nodes {
//	        name
//	        privacy
//	        description
//	        parentTeam {
//	          name
//	        }
//	        members(first: 30, membership: IMMEDIATE) {
//	          pageInfo {
//	            hasNextPage
//	            endCursor
//	          }
//	          nodes {
//	            login
//	          }
//	        }
//	        id
//	      }
//	    }
//	  }
//	}
type queryTeamsMembersResult struct {
	Organization struct {
		Teams teamMembersGraphQL `graphql:"teams(first: 50, after: $teamsCursor)"`
	} `graphql:"organization(login: $repositoryOwner)"`
}

// queryResultMemberLimitedAvailability was derived from
//
//	query {
//	 user(login: "$login") {
//	   status {
//	     indicatesLimitedAvailability
//	   }
//	 }
//	}
type queryResultMemberLimitedAvailability struct {
	User struct {
		Status struct {
			IndicatesLimitedAvailability githubv4.Boolean
		}
	} `graphql:"user(login: $login)"`
}
