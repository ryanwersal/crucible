package cli

import (
	"bytes"
	"io"
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

// testCmdWithScript builds a root command with an empty crucible.js in the source dir.
func testCmdWithScript(t *testing.T) (string, string, *bytes.Buffer, *bytes.Buffer, func(args ...string) error) {
	t.Helper()
	src := t.TempDir()
	tgt := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "crucible.js"), []byte(`// empty`), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, run := testCmdDirs(src, tgt)
	return src, tgt, stdout, stderr, run
}

func testCmdDirs(src, tgt string) (*bytes.Buffer, *bytes.Buffer, func(args ...string) error) {
	return testCmdDirsWithStdin(src, tgt, nil)
}

func testCmdDirsWithStdin(src, tgt string, stdin io.Reader) (*bytes.Buffer, *bytes.Buffer, func(args ...string) error) {
	var stdout, stderr bytes.Buffer
	opts := &rootOpts{source: src, target: tgt}
	cmd := buildRootCmd(opts)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if stdin != nil {
		cmd.SetIn(stdin)
	}
	run := func(args ...string) error {
		cmd.SetArgs(args)
		return cmd.Execute()
	}
	return &stdout, &stderr, run
}

func TestApplyCmd_DryRun_UpToDate(t *testing.T) {
	t.Parallel()
	_, _, stdout, _, run := testCmdWithScript(t)

	if err := run("apply", "--dry-run"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "Everything up to date") {
		t.Errorf("stdout = %q, want 'Everything up to date'", stdout.String())
	}
}

func TestApplyCmd_DryRun_NoScript_Fails(t *testing.T) {
	t.Parallel()
	_, _, run := testCmd(t)

	if err := run("apply", "--dry-run"); err == nil {
		t.Fatal("expected error when crucible.js is missing")
	}
}

func TestApplyCmd_DryRun_ShowsActions(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	tgt := t.TempDir()

	scriptContent := `
		var c = require("crucible");
		c.file("~/.testfile", { content: "hello" });
	`
	if err := os.WriteFile(filepath.Join(src, "crucible.js"), []byte(scriptContent), 0o644); err != nil {
		t.Fatal(err)
	}

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

	scriptContent := `
		var c = require("crucible");
		c.file("~/.testfile", { content: "hello" });
	`
	if err := os.WriteFile(filepath.Join(src, "crucible.js"), []byte(scriptContent), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, run := testCmdDirs(src, tgt)

	if err := run("apply", "--dry-run"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(tgt, ".testfile")); err == nil {
		t.Fatal("dry run should not create files")
	}
}

func TestApplyCmd_CreatesFiles(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	tgt := t.TempDir()

	scriptContent := `
		var c = require("crucible");
		c.file("~/.testfile", { content: "applied", mode: 420 });
	`
	if err := os.WriteFile(filepath.Join(src, "crucible.js"), []byte(scriptContent), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, run := testCmdDirs(src, tgt)

	if err := run("apply", "--yes"); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(tgt, ".testfile"))
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

func TestShebangInvocation_MatchesApply(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	tgt := t.TempDir()

	script := "#!/usr/bin/env crucible\nvar c = require('crucible');\nc.file('~/.testfile', { content: 'shebang' });\n"
	scriptPath := filepath.Join(src, "crucible.js")
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	// Run via shebang-style: the script path as first arg, simulating OS shebang rewrite.
	stdout1, _, run1 := testCmdDirs(src, tgt)
	args := RewriteScriptArgs([]string{scriptPath, "--dry-run"}, SubcommandNames(buildRootCmd(&rootOpts{source: src, target: tgt})))
	if err := run1(args...); err != nil {
		t.Fatalf("shebang-style invocation failed: %v", err)
	}

	// Run via explicit apply.
	stdout2, _, run2 := testCmdDirs(src, tgt)
	if err := run2("apply", "--file", scriptPath, "--dry-run"); err != nil {
		t.Fatalf("explicit apply failed: %v", err)
	}

	if stdout1.String() != stdout2.String() {
		t.Errorf("outputs differ:\n  shebang: %q\n  apply:   %q", stdout1.String(), stdout2.String())
	}
}

func writeTestScript(t *testing.T, src string) {
	t.Helper()
	scriptContent := `
		var c = require("crucible");
		c.file("~/.testfile", { content: "applied", mode: 420 });
	`
	if err := os.WriteFile(filepath.Join(src, "crucible.js"), []byte(scriptContent), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestApplyCmd_ConfirmYes(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	tgt := t.TempDir()
	writeTestScript(t, src)

	_, _, run := testCmdDirsWithStdin(src, tgt, strings.NewReader("y\n"))

	if err := run("apply"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(tgt, ".testfile")); err != nil {
		t.Fatal("file should have been created after confirming 'y'")
	}
}

func TestApplyCmd_ConfirmYesFull(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	tgt := t.TempDir()
	writeTestScript(t, src)

	_, _, run := testCmdDirsWithStdin(src, tgt, strings.NewReader("yes\n"))

	if err := run("apply"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(tgt, ".testfile")); err != nil {
		t.Fatal("file should have been created after confirming 'yes'")
	}
}

func TestApplyCmd_ConfirmNo(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	tgt := t.TempDir()
	writeTestScript(t, src)

	_, stderr, run := testCmdDirsWithStdin(src, tgt, strings.NewReader("n\n"))

	if err := run("apply"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(tgt, ".testfile")); err == nil {
		t.Fatal("file should not have been created after declining")
	}
	if !strings.Contains(stderr.String(), "Aborted") {
		t.Errorf("stderr = %q, want 'Aborted'", stderr.String())
	}
}

func TestApplyCmd_ConfirmEmpty(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	tgt := t.TempDir()
	writeTestScript(t, src)

	_, stderr, run := testCmdDirsWithStdin(src, tgt, strings.NewReader("\n"))

	if err := run("apply"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(tgt, ".testfile")); err == nil {
		t.Fatal("file should not have been created with empty confirmation")
	}
	if !strings.Contains(stderr.String(), "Aborted") {
		t.Errorf("stderr = %q, want 'Aborted'", stderr.String())
	}
}

func TestApplyCmd_ConfirmEOF(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	tgt := t.TempDir()
	writeTestScript(t, src)

	_, stderr, run := testCmdDirsWithStdin(src, tgt, strings.NewReader(""))

	if err := run("apply"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(tgt, ".testfile")); err == nil {
		t.Fatal("file should not have been created on EOF")
	}
	if !strings.Contains(stderr.String(), "Aborted") {
		t.Errorf("stderr = %q, want 'Aborted'", stderr.String())
	}
}
