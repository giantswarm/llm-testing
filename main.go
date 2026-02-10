package main

import (
	"github.com/giantswarm/llm-testing/cmd"
)

// version will be set by goreleaser during build.
var version = "dev"

func main() {
	cmd.SetVersion(version)
	cmd.Execute()
}
