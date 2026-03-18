package engine

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
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

func TestPlan_NewFiles(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	tgt := t.TempDir()

	mustWriteFile(t, filepath.Join(src, "hello.txt"), []byte("hello"), 0o644)
	mustMkdirAll(t, filepath.Join(src, "subdir"), 0o755)
	mustWriteFile(t, filepath.Join(src, "subdir", "nested.txt"), []byte("nested"), 0o644)

	eng := New(src, tgt, slog.New(slog.DiscardHandler))
	result, err := eng.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Actions) == 0 {
		t.Fatal("expected actions for new files")
	}

	hasWrite := false
	hasDir := false
	for _, a := range result.Actions {
		switch a.Type.String() {
		case "WriteFile":
			hasWrite = true
		case "CreateDir":
			hasDir = true
		}
	}
	if !hasWrite {
		t.Fatal("expected WriteFile action")
	}
	if !hasDir {
		t.Fatal("expected CreateDir action")
	}
}

func TestPlan_Idempotent(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	tgt := t.TempDir()

	mustWriteFile(t, filepath.Join(src, "test.txt"), []byte("hello"), 0o644)

	eng := New(src, tgt, slog.New(slog.DiscardHandler))
	if _, err := eng.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}

	result, err := eng.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Actions) != 0 {
		t.Fatalf("expected 0 actions on second plan, got %d: %v", len(result.Actions), result.Actions)
	}
	// 2 observations: the target dir itself + the file
	if len(result.Observations) != 2 {
		t.Fatalf("expected 2 observations on second plan, got %d", len(result.Observations))
	}
}

func TestPlan_SkipsDotfiles(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	tgt := t.TempDir()

	mustWriteFile(t, filepath.Join(src, ".hidden"), []byte("secret"), 0o644)
	mustMkdirAll(t, filepath.Join(src, ".git"), 0o755)
	mustWriteFile(t, filepath.Join(src, ".git", "config"), []byte("git"), 0o644)

	eng := New(src, tgt, slog.New(slog.DiscardHandler))
	result, err := eng.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Actions) != 0 {
		t.Fatalf("expected 0 actions (dotfiles skipped), got %d", len(result.Actions))
	}
}

func TestPlan_SkipsCrucibleYaml(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	tgt := t.TempDir()

	mustWriteFile(t, filepath.Join(src, "crucible.yaml"), []byte("config: true"), 0o644)
	mustWriteFile(t, filepath.Join(src, "real.txt"), []byte("real"), 0o644)

	eng := New(src, tgt, slog.New(slog.DiscardHandler))
	result, err := eng.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	for _, a := range result.Actions {
		if filepath.Base(a.Path) == "crucible.yaml" {
			t.Fatal("crucible.yaml should be skipped")
		}
	}
}

func TestPlan_DetectsContentChange(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	tgt := t.TempDir()

	mustWriteFile(t, filepath.Join(src, "test.txt"), []byte("v1"), 0o644)

	eng := New(src, tgt, slog.New(slog.DiscardHandler))
	if _, err := eng.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}

	mustWriteFile(t, filepath.Join(src, "test.txt"), []byte("v2"), 0o644)

	result, err := eng.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	hasWrite := false
	for _, a := range result.Actions {
		if a.Type.String() == "WriteFile" {
			hasWrite = true
		}
	}
	if !hasWrite {
		t.Fatal("expected WriteFile action for changed content")
	}
}

func TestApply_CreatesFiles(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	tgt := t.TempDir()

	mustWriteFile(t, filepath.Join(src, "test.txt"), []byte("hello"), 0o644)

	eng := New(src, tgt, slog.New(slog.DiscardHandler))
	if _, err := eng.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(filepath.Join(tgt, "test.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "hello" {
		t.Fatalf("expected 'hello', got %q", content)
	}
}

// TestPlan_BackwardCompat verifies that source dirs without crucible.js
// still use the WalkDir-based plan (no script needed).
func TestPlan_BackwardCompat(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	tgt := t.TempDir()

	mustWriteFile(t, filepath.Join(src, "config.txt"), []byte("value"), 0o644)

	eng := New(src, tgt, slog.New(slog.DiscardHandler))
	result, err := eng.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(result.Actions))
	}
	if result.Actions[0].Type.String() != "WriteFile" {
		t.Errorf("expected WriteFile, got %s", result.Actions[0].Type.String())
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
		switch a.Type.String() {
		case "WriteFile":
			hasWrite = true
		case "CreateDir":
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
