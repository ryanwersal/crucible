package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyPlanVisibility(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                   string
		flags                  []string
		details, apply, prompt bool
	}{
		{name: "default", prompt: true},
		{name: "show plan", flags: []string{"--show-plan"}, details: true, prompt: true},
		{name: "dry run", flags: []string{"--dry-run"}},
		{name: "dry run with plan", flags: []string{"--dry-run", "--show-plan"}, details: true},
		{name: "yes", flags: []string{"--yes"}, apply: true},
		{name: "yes with plan", flags: []string{"--yes", "--show-plan"}, details: true, apply: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			src, target := t.TempDir(), t.TempDir()
			script := `var c = require("crucible"); c.file("~/new-file", {content:"new"}); c.file("~/existing-file", {content:"same"});`
			if err := os.WriteFile(filepath.Join(src, "crucible.js"), []byte(script), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(target, "existing-file"), []byte("same"), 0o644); err != nil {
				t.Fatal(err)
			}
			out, errout, run := testCmdDirsWithStdin(src, target, strings.NewReader("n\n"))
			if err := run(append([]string{"apply"}, tc.flags...)...); err != nil {
				t.Fatal(err)
			}
			if strings.Count(out.String(), "new-file") != 1 {
				t.Fatalf("pending action must appear exactly once: %s", out)
			}
			if strings.Contains(out.String(), "existing-file") != tc.details {
				t.Fatalf("unexpected unchanged-resource visibility: %s", out)
			}

			if strings.Contains(errout.String(), "new-file") {
				t.Fatalf("duplicate action tree: %s", errout)
			}
			if strings.Contains(errout.String(), "Proceed?") != tc.prompt {
				t.Fatalf("unexpected confirmation: %s", errout)
			}
			if _, err := os.Stat(filepath.Join(target, "new-file")); (err == nil) != tc.apply {
				t.Fatalf("unexpected apply state: %v", err)
			}
		})
	}
}

func TestApplyQuietNoChanges(t *testing.T) {
	t.Parallel()
	src, target := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "existing"), []byte("same"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "crucible.js"), []byte(`var c=require("crucible"); c.file("~/existing", {content:"same"});`), 0o644); err != nil {
		t.Fatal(err)
	}
	out, errout, run := testCmdDirs(src, target)
	if err := run("apply"); err != nil {
		t.Fatal(err)
	}
	if out.String() != "Everything up to date.\n" || errout.Len() != 0 {
		t.Fatalf("unexpected output: %q %q", out, errout)
	}
}

func TestApplyQuietPreservesDestructiveConfirmation(t *testing.T) {
	t.Parallel()
	src, target := t.TempDir(), t.TempDir()
	path := filepath.Join(target, "valuable-file")
	if err := os.WriteFile(path, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}
	script := `var c=require("crucible"); c.symlink("~/valuable-file", {target:"replacement"});`
	if err := os.WriteFile(filepath.Join(src, "crucible.js"), []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	out, errout, run := testCmdDirsWithStdin(src, target, strings.NewReader("no\n"))
	if err := run("apply", "--yes"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), path) || !strings.Contains(errout.String(), "Type \"yes\" to confirm destruction") || !strings.Contains(errout.String(), path) {
		t.Fatalf("missing destructive confirmation: %q %q", out, errout)
	}
	content, err := os.ReadFile(path)
	if err != nil || string(content) != "keep me" {
		t.Fatalf("declined destruction changed file: %q %v", content, err)
	}
}
