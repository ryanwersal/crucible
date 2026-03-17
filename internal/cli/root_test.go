package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testCmd builds a root command with source/target pointed at temp dirs.
func testCmd(t *testing.T) (*bytes.Buffer, *bytes.Buffer, func(args ...string) error) {
	t.Helper()
	src := t.TempDir()
	tgt := t.TempDir()
	return testCmdDirs(src, tgt)
}

func testCmdDirs(src, tgt string) (*bytes.Buffer, *bytes.Buffer, func(args ...string) error) {
	var stdout, stderr bytes.Buffer
	opts := &rootOpts{source: src, target: tgt}
	cmd := buildRootCmd(opts)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	run := func(args ...string) error {
		cmd.SetArgs(args)
		return cmd.Execute()
	}
	return &stdout, &stderr, run
}

func TestApplyCmd_DryRun_UpToDate(t *testing.T) {
	t.Parallel()
	stdout, _, run := testCmd(t)

	if err := run("apply", "--dry-run"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "Everything up to date") {
		t.Errorf("stdout = %q, want 'Everything up to date'", stdout.String())
	}
}

func TestApplyCmd_DryRun_ShowsActions(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	tgt := t.TempDir()
	os.WriteFile(filepath.Join(src, "hello.txt"), []byte("hello"), 0o644)

	stdout, _, run := testCmdDirs(src, tgt)

	if err := run("apply", "--dry-run"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "action(s) would be taken") {
		t.Errorf("stdout = %q, want action count", stdout.String())
	}
}

func TestApplyCmd_DryRun_NoChanges(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	tgt := t.TempDir()
	os.WriteFile(filepath.Join(src, "test.txt"), []byte("test"), 0o644)

	_, _, run := testCmdDirs(src, tgt)

	if err := run("apply", "--dry-run"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(tgt, "test.txt")); err == nil {
		t.Fatal("dry run should not create files")
	}
}

func TestApplyCmd_CreatesFiles(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	tgt := t.TempDir()
	os.WriteFile(filepath.Join(src, "test.txt"), []byte("applied"), 0o644)

	_, _, run := testCmdDirs(src, tgt)

	if err := run("apply"); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(tgt, "test.txt"))
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if string(content) != "applied" {
		t.Errorf("content = %q, want 'applied'", content)
	}
}

func TestRootCmd_NoSourceTargetFlags(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	if f := cmd.PersistentFlags().Lookup("source"); f != nil {
		t.Error("--source flag should not exist")
	}
	if f := cmd.PersistentFlags().Lookup("target"); f != nil {
		t.Error("--target flag should not exist")
	}
}

func TestApplyCmd_UnknownFlag(t *testing.T) {
	t.Parallel()
	_, _, run := testCmd(t)
	if err := run("apply", "--bogus"); err == nil {
		t.Error("expected error for unknown flag")
	}
}
