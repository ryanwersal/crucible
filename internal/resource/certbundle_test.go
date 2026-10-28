package resource

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryanwersal/crucible/internal/action"
)

func TestCertBundleSudoBoundary(t *testing.T) {
	// PATH is process-global: do not run this test in parallel.
	dir := t.TempDir()
	log := filepath.Join(dir, "args")
	fake := `#!/bin/sh
printf '%s\n' "$@" > "$CERT_BUNDLE_TEST_ARGS"
`
	if err := os.WriteFile(filepath.Join(dir, "sudo"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("CERT_BUNDLE_TEST_ARGS", log)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	a := action.Action{Type: action.FilterCertBundle, Path: "/tmp/bundle with spaces.pem", CertSnapshot: strings.Repeat("A", 64), CertAbsent: []string{strings.Repeat("B", 64)}, NeedsSudo: true}
	if err := (FilterCertBundleExecutor{}).Execute(context.Background(), a, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{"-p", "crucible needs administrator privileges. Password: ", "--", exe, "_filter-cert-bundle", a.Path, a.CertSnapshot, a.CertAbsent[0], ""}, "\n")
	if string(got) != want {
		t.Fatalf("privileged command args = %q, want %q", got, want)
	}
}
