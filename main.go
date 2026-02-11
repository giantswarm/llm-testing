package main

import (
	"github.com/giantswarm/llm-testing/cmd"
)

// These will be set by goreleaser during build via ldflags.
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	cmd.SetVersion(version)
	cmd.SetBuildInfo(commit, date)
	cmd.Execute()
}
