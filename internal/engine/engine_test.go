package engine

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestPlan_NewFiles(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	tgt := t.TempDir()

	os.WriteFile(filepath.Join(src, "hello.txt"), []byte("hello"), 0o644)
	os.MkdirAll(filepath.Join(src, "subdir"), 0o755)
	os.WriteFile(filepath.Join(src, "subdir", "nested.txt"), []byte("nested"), 0o644)

	eng := New(src, tgt, slog.New(slog.DiscardHandler))
	actions, err := eng.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(actions) == 0 {
		t.Fatal("expected actions for new files")
	}

	hasWrite := false
	hasDir := false
	for _, a := range actions {
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

	content := []byte("hello")
	os.WriteFile(filepath.Join(src, "test.txt"), content, 0o644)

	eng := New(src, tgt, slog.New(slog.DiscardHandler))

	if err := eng.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}

	actions, err := eng.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 0 {
		t.Fatalf("expected 0 actions on second plan, got %d: %v", len(actions), actions)
	}
}

func TestPlan_SkipsDotfiles(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	tgt := t.TempDir()

	os.WriteFile(filepath.Join(src, ".hidden"), []byte("secret"), 0o644)
	os.MkdirAll(filepath.Join(src, ".git"), 0o755)
	os.WriteFile(filepath.Join(src, ".git", "config"), []byte("git"), 0o644)

	eng := New(src, tgt, slog.New(slog.DiscardHandler))
	actions, err := eng.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(actions) != 0 {
		t.Fatalf("expected 0 actions (dotfiles skipped), got %d", len(actions))
	}
}

func TestPlan_SkipsCrucibleYaml(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	tgt := t.TempDir()

	os.WriteFile(filepath.Join(src, "crucible.yaml"), []byte("config: true"), 0o644)
	os.WriteFile(filepath.Join(src, "real.txt"), []byte("real"), 0o644)

	eng := New(src, tgt, slog.New(slog.DiscardHandler))
	actions, err := eng.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	for _, a := range actions {
		if filepath.Base(a.Path) == "crucible.yaml" {
			t.Fatal("crucible.yaml should be skipped")
		}
	}
}

func TestPlan_DetectsContentChange(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	tgt := t.TempDir()

	os.WriteFile(filepath.Join(src, "test.txt"), []byte("v1"), 0o644)

	eng := New(src, tgt, slog.New(slog.DiscardHandler))
	eng.Apply(context.Background())

	os.WriteFile(filepath.Join(src, "test.txt"), []byte("v2"), 0o644)

	actions, err := eng.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	hasWrite := false
	for _, a := range actions {
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

	os.WriteFile(filepath.Join(src, "test.txt"), []byte("hello"), 0o644)

	eng := New(src, tgt, slog.New(slog.DiscardHandler))
	if err := eng.Apply(context.Background()); err != nil {
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
