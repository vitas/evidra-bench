package main

import (
	"fmt"
	"os"

	"github.com/vitas/evidra-bench/pkg/harness"
)

var (
	version = "dev"
	commit  = "dev"
	date    = "dev"
)

func buildVersionString() string {
	return fmt.Sprintf("bench-cli %s (commit: %s, built: %s)", version, commit, date)
}

func main() {
	harness.SetVersion(version, commit)
	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
