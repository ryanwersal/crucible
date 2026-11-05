package resource

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryanwersal/crucible/internal/action"
)

func TestAddHomebrewTapTrustsBeforeTapping(t *testing.T) {
	for _, fail := range []string{"", "trust", "tap"} {
		name := fail
		if name == "" {
			name = "success"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			log := filepath.Join(dir, "calls")
			t.Setenv("PATH", dir)
			t.Setenv("CRUCIBLE_TEST_TAP_CALLS", log)
			t.Setenv("CRUCIBLE_TEST_TAP_FAIL", fail)
			script := `#!/bin/sh
printf '%s\n' "$*" >> "$CRUCIBLE_TEST_TAP_CALLS"
if [ "$1" = "$CRUCIBLE_TEST_TAP_FAIL" ]; then exit 42; fi
`
			if err := os.WriteFile(filepath.Join(dir, "brew"), []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			err := (AddHomebrewTapExecutor{}).Execute(t.Context(), action.Action{TapName: "owner/tools"}, nil, io.Discard, io.Discard)
			if (err != nil) != (fail != "") {
				t.Fatalf("error = %v, failing command = %q", err, fail)
			}
			calls, readErr := os.ReadFile(log)
			if readErr != nil {
				t.Fatal(readErr)
			}
			want := "trust --tap owner/tools\ntap owner/tools\n"
			if fail == "trust" {
				want = "trust --tap owner/tools\n"
			}
			if string(calls) != want {
				t.Fatalf("commands = %q, want %q", calls, want)
			}
			if err != nil && !strings.Contains(err.Error(), fail) {
				t.Fatalf("failure omitted command: %v", err)
			}
		})
	}
}
