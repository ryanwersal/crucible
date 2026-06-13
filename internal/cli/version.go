package cli

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// These are overridden at release time via -ldflags -X (see .goreleaser.yml).
// For local `go build`/`go install` they stay at the defaults below and are
// backfilled from the VCS data the Go toolchain embeds in every binary, so
// local builds still identify their source commit, build time, and dirty state.
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

// versionInfo resolves the version triple, reading the embedded build metadata.
func versionInfo() (v, c, d string) {
	var settings []debug.BuildSetting
	if info, ok := debug.ReadBuildInfo(); ok {
		settings = info.Settings
	}
	return resolveVersion(version, commit, date, settings)
}

// resolveVersion prefers ldflags-injected values (release builds) and otherwise
// backfills commit and date from the toolchain's embedded VCS settings. A
// commit other than "unknown" means the linker stamped the build, so those
// values win unchanged.
func resolveVersion(version, commit, date string, settings []debug.BuildSetting) (string, string, string) {
	if commit != "unknown" {
		return version, commit, date
	}

	var revision, vcsTime, modified string
	for _, s := range settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.time":
			vcsTime = s.Value
		case "vcs.modified":
			modified = s.Value
		}
	}

	if revision != "" {
		if len(revision) > 7 {
			revision = revision[:7]
		}
		if modified == "true" {
			revision += "-dirty"
		}
		commit = revision
	}
	if vcsTime != "" {
		date = vcsTime
	}
	return version, commit, date
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			v, c, d := versionInfo()
			fmt.Printf("crucible %s\n", v)
			fmt.Printf("  commit: %s\n", c)
			fmt.Printf("  built:  %s\n", d)
		},
	}
}
