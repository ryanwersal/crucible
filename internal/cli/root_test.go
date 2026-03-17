package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlanCmd_UpToDate(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	tgt := t.TempDir()

	var stdout bytes.Buffer
	cmd := NewRootCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"plan", "--source", src, "--target", tgt})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if !strings.Contains(stdout.String(), "Everything up to date") {
		t.Errorf("stdout = %q, want 'Everything up to date'", stdout.String())
	}
}

func TestPlanCmd_ShowsActions(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	tgt := t.TempDir()

	os.WriteFile(filepath.Join(src, "hello.txt"), []byte("hello"), 0o644)

	var stdout bytes.Buffer
	cmd := NewRootCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"plan", "--source", src, "--target", tgt})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if !strings.Contains(stdout.String(), "action(s) would be taken") {
		t.Errorf("stdout = %q, want action count", stdout.String())
	}
}

func TestApplyCmd_DryRun(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	tgt := t.TempDir()

	os.WriteFile(filepath.Join(src, "test.txt"), []byte("test"), 0o644)

	var stdout bytes.Buffer
	cmd := NewRootCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"apply", "--dry-run", "--source", src, "--target", tgt})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if !strings.Contains(stdout.String(), "dry run") {
		t.Errorf("stdout = %q, want 'dry run'", stdout.String())
	}

	// Target should not have the file
	if _, err := os.Stat(filepath.Join(tgt, "test.txt")); err == nil {
		t.Fatal("dry run should not create files")
	}
}

func TestApplyCmd_CreatesFiles(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	tgt := t.TempDir()

	os.WriteFile(filepath.Join(src, "test.txt"), []byte("applied"), 0o644)

	var stdout bytes.Buffer
	cmd := NewRootCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"apply", "--source", src, "--target", tgt})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tgt, "test.txt"))
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if string(content) != "applied" {
		t.Errorf("content = %q, want 'applied'", content)
	}
}
