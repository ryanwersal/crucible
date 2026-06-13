package cli

import (
	"runtime/debug"
	"testing"
)

func TestResolveVersion(t *testing.T) {
	t.Parallel()

	vcs := []debug.BuildSetting{
		{Key: "vcs.revision", Value: "1234567890abcdef"},
		{Key: "vcs.time", Value: "2026-06-13T00:00:00Z"},
		{Key: "vcs.modified", Value: "false"},
	}

	tests := []struct {
		name                          string
		version, commit, date         string
		settings                      []debug.BuildSetting
		wantVer, wantCommit, wantDate string
	}{
		{
			name: "ldflags stamped — VCS ignored",
			// A release build sets commit, so those values win even if VCS data exists.
			version: "v1.2.3", commit: "abc1234", date: "2026-01-01",
			settings: vcs,
			wantVer:  "v1.2.3", wantCommit: "abc1234", wantDate: "2026-01-01",
		},
		{
			name:    "local build — backfilled and shortened",
			version: "dev", commit: "unknown", date: "unknown",
			settings: vcs,
			wantVer:  "dev", wantCommit: "1234567", wantDate: "2026-06-13T00:00:00Z",
		},
		{
			name:    "local build dirty — dirty marker appended",
			version: "dev", commit: "unknown", date: "unknown",
			settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "1234567890abcdef"},
				{Key: "vcs.modified", Value: "true"},
			},
			wantVer: "dev", wantCommit: "1234567-dirty", wantDate: "unknown",
		},
		{
			name:    "no VCS info — defaults preserved",
			version: "dev", commit: "unknown", date: "unknown",
			settings: nil,
			wantVer:  "dev", wantCommit: "unknown", wantDate: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v, c, d := resolveVersion(tt.version, tt.commit, tt.date, tt.settings)
			if v != tt.wantVer || c != tt.wantCommit || d != tt.wantDate {
				t.Errorf("resolveVersion() = (%q, %q, %q), want (%q, %q, %q)",
					v, c, d, tt.wantVer, tt.wantCommit, tt.wantDate)
			}
		})
	}
}
