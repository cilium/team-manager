# Team manager

Team manager is a utility that allows an organization owner to add or remove
people from existing teams and / or assign people for [GitHub team review
assignments](https://docs.github.com/en/github/setting-up-and-managing-organizations-and-teams/managing-code-review-assignment-for-your-team).

## Features

- [X] Retrieve all teams, associated members and code review assignments in an
      organization into a configuration file.
- [X] Sync local configuration file into GitHub.
  - [X] Add and / or remove new members to / from teams.
  - [X] Exclude team members from code review assignments.
- [X] GitHub action (see example below)

## Missing features

- [ ] Retrieve excluded team members from the code review assignments
      (not provided by GitHub API).
- [ ] Create or delete teams that are added or removed from the local
      configuration file.

# Build

```bash
make team-manager
```

# Usage

1. Generate a GitHub token that has `admin:org`. [direct link](https://github.com/settings/tokens/new)

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
slackWorkspace: cilium.slack.com
# List of members that belong to the organization, ordered by GitHub login (username).
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
# List of teams that belong to the organization, ordered by team names.
teams:
  bpf:
    # team ID, retrieved from GitHub
    id: MDQ6VGVhbTI1MTk3Nzk=
    # List of members' logins that belong to this team.
    members:
    - aanm
    - borkmann
    - joestringer
    # codeReviewAssignment
    codeReviewAssignment:
      # algorithm, currently can be LOAD_BALANCE or ROUND_ROBIN.
      algorithm: LOAD_BALANCE
      # set 'true' if codeReviewAssignment should be enabled.
      enabled: true
      # Notify the entire team of the PR if it is delegated.
      notifyTeam: false
      # List of members that should be excluded from receiving reviews, and an
      # optional reason.
      excludedMembers:
        # GitHub login name (username).
      - login: aanm
        reason: Want to be part of team 'bpf' but will not be assigned to leave
                reviews.
      # The number of team members to assign.
      teamMemberCount: 1
  policy:
    id: MDQ6VGVhbTI1MTk3ODY=
    members:
    - aanm
    - joestringer
    codeReviewAssignment:
      algorithm: LOAD_BALANCE
      enabled: true
      notifyTeam: true
      teamMemberCount: 1
# List of members that should be excluded from review assignments for the teams
# that they belong. This list can exist for numerous reasons, person is
# currently PTO or busy with other work.
excludeCodeReviewAssignmentFromAllTeams:
- borkmann
```

4. Once the changes stored in a local configuration file, run `./team-manager push --org cilium`:

```bash
$ ./team-manager push --org cilium
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

# GitHub action

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
      - uses: actions/checkout@v1
      - uses: docker://quay.io/cilium/team-manager:v0.0.1
        name: Sync team
        with:
          entrypoint: team-manager
          args: push --force --config-filename ./team-assignments.yaml
        env:
          GITHUB_TOKEN: ${{ secrets.ADMIN_ORG_TOKEN }}
```