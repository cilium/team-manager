module github.com/cilium/team-manager

go 1.20

require (
	github.com/google/go-github/v33 v33.0.0
	github.com/google/renameio v1.0.1
	github.com/kr/pretty v0.3.1
	github.com/pmezard/go-difflib v1.0.0
	github.com/shurcooL/githubv4 v0.0.0-20230424031643-6cea62ecd5a9
	github.com/spf13/cobra v1.7.0
	golang.org/x/oauth2 v0.9.0
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/rogpeppe/go-internal v1.10.0 // indirect
	github.com/shurcooL/graphql v0.0.0-20220606043923-3cf50f8a0a29 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	golang.org/x/crypto v0.10.0 // indirect
	golang.org/x/net v0.11.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
)

replace (
	github.com/shurcooL/githubv4 => github.com/aanm/githubv4 v0.0.0-20210126140237-7e156a79723b
	github.com/shurcooL/graphql => github.com/aanm/graphql v0.0.0-20210126135448-cdc0856bcf8b
)
