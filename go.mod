module eos-roadmap-tools

go 1.24.0

require github.com/shurcooL/githubv4 v0.0.0-20240628060444-f4e9a8529af8

require (
	github.com/shurcooL/graphql v0.0.0-20230722043721-ed46e5a46466 // indirect
	golang.org/x/oauth2 v0.31.0 // indirect
)

replace github.com/shurcooL/githubv4 => ./third_party/githubv4
