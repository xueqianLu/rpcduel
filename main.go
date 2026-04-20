package main

import "github.com/xueqianLu/rpcduel/cmd"

// Build info, populated via -ldflags by Makefile / GoReleaser.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cmd.SetBuildInfo(version, commit, date)
	cmd.Execute()
}
