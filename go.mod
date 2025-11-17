module github.com/cilium/team-manager

go 1.23

require (
	github.com/google/go-github/v67 v67.0.0
	github.com/google/renameio v1.0.1
	github.com/kr/pretty v0.3.1
	github.com/pmezard/go-difflib v1.0.0
	github.com/schollz/progressbar/v3 v3.14.1
	github.com/shurcooL/githubv4 v0.0.0-20240727222349-48295856cce7
	github.com/shurcooL/graphql v0.0.0-20230722043721-ed46e5a46466
	github.com/spf13/cobra v1.8.0
	golang.org/x/oauth2 v0.17.0
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/mitchellh/colorstring v0.0.0-20190213212951-d06e56a500db // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/rogpeppe/go-internal v1.12.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	golang.org/x/sys v0.17.0 // indirect
	golang.org/x/term v0.17.0 // indirect
	google.golang.org/appengine v1.6.8 // indirect
	google.golang.org/protobuf v1.32.0 // indirect
)

replace (
	github.com/shurcooL/githubv4 => github.com/aanm/githubv4 v0.0.0-20240213100002-d683ef4e8dad
	github.com/shurcooL/graphql => github.com/aanm/graphql v0.0.0-20240213100714-ff80a8740826
)
