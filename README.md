# Team manager

Team manager is a utility that allows an organization owner to:
- add or remove
people from existing teams and / or assign people for [GitHub team review
assignments](https://docs.github.com/en/github/setting-up-and-managing-organizations-and-teams/managing-code-review-assignment-for-your-team);
- configure repository permissions for teams and individual users;
- keep track of all "outside collaborators" and the reason why they are outside
  collaborators;
- check if all teams have enough reviewers;

## Features

- [X] Retrieve all teams, associated members and code review assignments in an
      organization into a configuration file.
- [X] Sync local configuration file into GitHub.
  - [X] Add and / or remove new members to / from teams.
  - [X] Exclude team members from code review assignments.
- [x] Create or delete teams that are added or removed from the local
      configuration file.
- [x] Sync team and user permissions of repositories.
- [x] Check Status of teams. Useful to know if teams have enough reviewers.
- [X] GitHub action (see example below)

## Missing features

- [ ] Retrieve excluded team members from the code review assignments
      (not provided by GitHub API).

# Build

```bash
make team-manager
```

# Usage

1. Generate a GitHub token that has `admin:org` and `public_repo`. [direct link](https://github.com/settings/tokens/new?description=Team%20Management&scopes=admin:org,public_repo)

2. Generate configuration for your organization

```bash
$ ./team-manager init --org cilium
Retrieving configuration from organization...
Creating configuration file "cilium-team-assignments.yaml"...
```

3. Modify your file accordingly the available options, for example (the yaml
   comments will not show up in the generated file and will be removed every time
   `./team-manager` is executed):

```yaml
organization: cilium
repositories:
  # Repository name
  cilium:
    # User-specific permissions. Always prefixed with 'USER'.
    # Valid options: 'USER-ADMIN', 'USER-MAINTAIN', 'USER-WRITE', 'USER-TRIAGE', 'USER-READ'
    USER-READ:
    - ciliumbot
    # Team-specific permissions.
    # Valid options: 'ADMIN', 'MAINTAIN', 'WRITE', 'TRIAGE', 'READ'
    WRITE:
    - bpf
    - sig-policy
members:
  aanm:
    # User ID, retrieved from GitHub
    id: MDQ6VXNlcjU3MTQwNjY=
    # User real name, useful to know which person is behind a GitHub username.
    name: André Martins
    # Slack user ID, to ping folks on Slack.
    slackId: U3Z10R6HW
  borkmann:
    id: MDQ6VXNlcjY3NzM5Mw==
    name: Daniel Borkmann
  joestringer:
    id: MDQ6VXNlcjEyNDMzMzY=
    name: Joe Stringer
# The list of 'outsideCollaborators' is automatically derived from the list of users that don't
# belong to the organization but have access to at least one of the repositories
# of the organization.
outsideCollaborators:
  ciliumbot:
    reason: "Only has access to some repositories"
# List of teams that belong to the organization.
teams:
  # Team Name
  Cilium Teams:
    # team ID, retrieved from GitHub
    id: T_kwDOAUFEZs4Ah5D3
    # team restID, retrieved from GitHub
    restID: 8884471
    # Team's description
    description: Teams and sigs used for Cilium projects
    # Team's privacy settings. Valid values: VISIBLE|SECRET
    privacy: VISIBLE
    # Teams that are children of this parent team
    children:
      # Team Name
      ebpf:
        # team ID, retrieved from GitHub
        id: MDQ6VGVhbTQ5MjY2ODE=
        # team restID, retrieved from GitHub
        restID: 4926681
        # Team's description
        description: All code related with ebpf.
        # List of members' logins that belong to this team.
        members:
        - aanm
        - borkmann
        - joestringer
        # Optional list of team mentors who will not be auto-assigned PRs for review
        mentors:
        - aanm
        codeReviewAssignment:
          # algorithm, currently can be LOAD_BALANCE or ROUND_ROBIN.
          algorithm: LOAD_BALANCE
          # set 'true' if codeReviewAssignment should be enabled.
          enabled: true
          # Notify the entire team of the PR if it is delegated.
          notifyTeam: false
          # List of members that should be excluded from receiving reviews, and
          # an optional reason.
          excludedMembers:
            # GitHub login name (username).
          - login: aanm
            reason: Want to be part of team 'bpf' but will not be assigned to leave
                    reviews.
          # The number of team members to assign.
          teamMemberCount: 1
        # Team's privacy settings. Valid values: VISIBLE|SECRET
        privacy: VISIBLE
  # Team Name
  policy:
    id: MDQ6VGVhbTI1MTk3ODY=
    restID: 8884472
    description: All control plane code related with Policy
    members:
    - aanm
    - joestringer
    codeReviewAssignment:
      algorithm: LOAD_BALANCE
      enabled: true
      notifyTeam: true
      teamMemberCount: 1
    privacy: SECRET
# List of members that should be excluded from review assignments for the teams
# that they belong. This list can exist for numerous reasons, person is
# currently PTO or busy with other work.
excludeCodeReviewAssignmentFromAllTeams:
- borkmann
```

4. Once the changes stored in a local configuration file, run `./team-manager push --org cilium`:

```bash
$ ./team-manager push --config-filename ./cilium-team-assignments.yaml
Local config out of sync with upstream: Unified diff:
--- local
+++ remote
@@ -1,11 +1,11 @@
 config.TeamConfig{
     ID:                   "MDQ6VGVhbTI1MTk3Nzk=",
-    Members:              {"aanm", "borkmann", "joestringer"},
+    Members:              {"borkmann", "joestringer"},,
     CodeReviewAssignment: config.CodeReviewAssignment{
         Algorithm:       "LOAD_BALANCE",
         Enabled:         true,
         ExcludedMembers: nil,
         NotifyTeam:      false,
         TeamMemberCount: 1,
     },
 }

Going to submit the following changes:
 Team: bpf
    Adding members: aanm
  Removing members: 
Continue? [y/n]: y
Adding member aanm to team bpf
Do you want to update CodeReviewAssignments? [y/n]: y
Excluding members from team: bpf
Excluding members from team: policy
```

# Repository and members sync

Starting with v1.0.0, team-manager has the ability to also sync repository and
members permissions. Before changing the file locally to push new changes,
it is important to always perform a 'sync':
```bash
$ ./team-manager sync --config-filename ./team-assignments.yaml
```

Then, after modifying the file on `team-assignments.yaml`, push the changes:

```bash
$ ./team-manager push --config-filename ./team-assignments.yaml
```

# GitHub action

On a large GitHub organization, it might be difficult to control who can create
repositories. Thus, when running team manager with a GitHub action, it should
run with `--repositories=false` and `--members=false` otherwise the GitHub
action might override repository permissions that were set from the web-ui
and remove or add members to the organization that were previously added or
removed from the web-ui.

```yaml
name: Team management
on:
  push:
    branches:
      - main

jobs:
  sync:
    # if: github.repository == '<my-org>/<my-repo>'
    name: Team sync
    runs-on: ubuntu-20.04
    steps:
      - uses: actions/checkout@b4ffde65f46336ab88eb53be808477a3936bae11 # v4.1.1
      - uses: docker://quay.io/cilium/team-manager:v1.0.0
        name: Sync team
        with:
          entrypoint: team-manager
          # With --repositories=false --members=false, it will have the same
          # behavior as <= v0.0.8.
          args: push --force --repositories=false --members=false --config-filename ./team-assignments.yaml
        env:
          GITHUB_TOKEN: ${{ secrets.ADMIN_ORG_TOKEN }}
```

# Check number of reviewers

To disable code review assignments, the GitHub user can either set its status as
'busy' or the team maintainer can exclude them from the list of reviewers.

Since the team maintainer can't control the status of the user, it is important
to retrieve the list of teams that don't have enough reviewers, by checking
which users have their status as 'busy' or the ones that are excluded from
reviews.

```bash
$ ./team-manager status --config-filename ./cilium-team-assignments.yaml
Checking status of "aanm"
Checking status of "joestringer"
Checking status of "borkmann"
Team "bpf" with 3 members doesn't have enough reviewers:
 - aanm - excluded
 - joestringer - busy
 - borkmann - ok
```

# Upgrade from <=0.0.8 to 1.0.0

1. Use 'sync' to sync the upstream configuration with the local file. It will
   fetch the information from GitHub and merge it with the local file.

```bash
$ ./team-manager sync --config-filename ./team-assignments.yaml
```
