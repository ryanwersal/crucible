package engine

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/script"
)

func mustWriteFile(t *testing.T, path string, data []byte, perm os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, perm); err != nil {
		t.Fatal(err)
	}
}

func mustMkdirAll(t *testing.T, path string, perm os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(path, perm); err != nil {
		t.Fatal(err)
	}
}

func TestPlan_NoScript_Fails(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	tgt := t.TempDir()

	eng := New(src, tgt, slog.New(slog.DiscardHandler))
	_, err := eng.Plan(context.Background())
	if err == nil {
		t.Fatal("expected error when crucible.js is missing")
	}
	if !errors.Is(err, script.ErrNoScript) {
		t.Errorf("expected ErrNoScript, got: %v", err)
	}
}

func TestPlan_ExplicitScriptFile_NotFound(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	tgt := t.TempDir()

	eng := New(src, tgt, slog.New(slog.DiscardHandler))
	eng.SetScriptFile(filepath.Join(src, "nonexistent.js"))
	_, err := eng.Plan(context.Background())
	if err == nil {
		t.Fatal("expected error for missing explicit script file")
	}
}

// TestPlan_Script verifies script-driven planning with crucible.js.
func TestPlan_Script(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	tgt := t.TempDir()

	scriptContent := `
		var c = require("crucible");
		c.file("~/.bashrc", { content: "# managed by crucible" });
		c.dir("~/.config", { mode: 493 });
	`
	mustWriteFile(t, filepath.Join(src, "crucible.js"), []byte(scriptContent), 0o644)

	eng := New(src, tgt, slog.New(slog.DiscardHandler))
	result, err := eng.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	hasWrite := false
	hasDir := false
	for _, a := range result.Actions {
		switch a.Type {
		case action.WriteFile:
			hasWrite = true
		case action.CreateDir:
			hasDir = true
		}
	}
	if !hasWrite {
		t.Fatal("expected WriteFile action from script")
	}
	if !hasDir {
		t.Fatal("expected CreateDir action from script")
	}
}

// TestApply_Script verifies end-to-end script-driven apply.
func TestApply_Script(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	tgt := t.TempDir()

	scriptContent := `
		var c = require("crucible");
		c.file("~/.bashrc", { content: "# managed by crucible", mode: 420 });
	`
	mustWriteFile(t, filepath.Join(src, "crucible.js"), []byte(scriptContent), 0o644)

	eng := New(src, tgt, slog.New(slog.DiscardHandler))
	if _, err := eng.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(filepath.Join(tgt, ".bashrc"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "# managed by crucible" {
		t.Fatalf("expected '# managed by crucible', got %q", content)
	}
}

// TestPlan_Script_SourceFile verifies source file references in scripts.
func TestPlan_Script_SourceFile(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	tgt := t.TempDir()

	mustMkdirAll(t, filepath.Join(src, "fish"), 0o755)
	mustWriteFile(t, filepath.Join(src, "fish", "config.fish"), []byte("set PATH /usr/local/bin"), 0o644)

	scriptContent := `
		var c = require("crucible");
		c.file("~/.config/fish/config.fish", { source: "fish/config.fish" });
	`
	mustWriteFile(t, filepath.Join(src, "crucible.js"), []byte(scriptContent), 0o644)

	eng := New(src, tgt, slog.New(slog.DiscardHandler))
	if _, err := eng.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(filepath.Join(tgt, ".config", "fish", "config.fish"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "set PATH /usr/local/bin" {
		t.Fatalf("got %q", content)
	}
}

// TestPlan_Script_Idempotent verifies that a second plan produces no actions
// and produces observations for each managed item.
func TestPlan_Script_Idempotent(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	tgt := t.TempDir()

	scriptContent := `
		var c = require("crucible");
		c.file("~/.bashrc", { content: "hello", mode: 420 });
	`
	mustWriteFile(t, filepath.Join(src, "crucible.js"), []byte(scriptContent), 0o644)

	eng := New(src, tgt, slog.New(slog.DiscardHandler))
	if _, err := eng.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}

	result, err := eng.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Actions) != 0 {
		t.Fatalf("expected 0 actions on second plan, got %d", len(result.Actions))
	}
	if len(result.Observations) != 1 {
		t.Fatalf("expected 1 observation on second plan, got %d", len(result.Observations))
	}
}

// TestPlan_ExplicitScriptFile verifies that SetScriptFile overrides discovery.
func TestPlan_ExplicitScriptFile(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	tgt := t.TempDir()

	// Put the script in a non-standard location
	altDir := t.TempDir()
	scriptContent := `
		var c = require("crucible");
		c.file("~/.vimrc", { content: "set nocompatible" });
	`
	mustWriteFile(t, filepath.Join(altDir, "my-config.js"), []byte(scriptContent), 0o644)

	eng := New(src, tgt, slog.New(slog.DiscardHandler))
	eng.SetScriptFile(filepath.Join(altDir, "my-config.js"))
	result, err := eng.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Actions) == 0 {
		t.Fatal("expected actions from explicit script file")
	}
}
