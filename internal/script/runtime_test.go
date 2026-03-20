package script

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ryanwersal/crucible/internal/fact"
)

func TestRuntime_ExecuteBasic(t *testing.T) {
	t.Parallel()

	src := t.TempDir()
	tgt := t.TempDir()

	scriptContent := []byte(`
		var c = require("crucible");
		c.file("~/.bashrc", { content: "# my bashrc" });
		c.dir("~/.config", { mode: 0o700 });
		c.brew("ripgrep");
	`)

	if err := os.WriteFile(filepath.Join(src, "crucible.js"), scriptContent, 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	store := fact.NewStore()
	_, _ = fact.Get(ctx, store, "os", fact.OSCollector{})

	rt := NewRuntime(ctx, slog.New(slog.DiscardHandler), src, tgt, store)
	decls, err := rt.Execute(ctx, "crucible.js", scriptContent)
	if err != nil {
		t.Fatal(err)
	}

	if len(decls) != 3 {
		t.Fatalf("expected 3 declarations, got %d", len(decls))
	}

	if decls[0].Type != DeclFile {
		t.Errorf("decl[0].Type = %v, want DeclFile", decls[0].Type)
	}
	if string(decls[0].Content) != "# my bashrc" {
		t.Errorf("decl[0].Content = %q", decls[0].Content)
	}

	if decls[1].Type != DeclDir {
		t.Errorf("decl[1].Type = %v, want DeclDir", decls[1].Type)
	}
	if decls[1].Mode != 0o700 {
		t.Errorf("decl[1].Mode = %o, want 700", decls[1].Mode)
	}

	if decls[2].Type != DeclPackage {
		t.Errorf("decl[2].Type = %v, want DeclPackage", decls[2].Type)
	}
	if decls[2].PackageName != "ripgrep" {
		t.Errorf("decl[2].PackageName = %q", decls[2].PackageName)
	}
}

func TestRuntime_Facts(t *testing.T) {
	t.Parallel()

	src := t.TempDir()
	tgt := t.TempDir()

	scriptContent := []byte(`
		var c = require("crucible");
		c.file("~/os.txt", { content: c.facts.os.name });
	`)

	ctx := context.Background()
	store := fact.NewStore()
	_, _ = fact.Get(ctx, store, "os", fact.OSCollector{})

	rt := NewRuntime(ctx, slog.New(slog.DiscardHandler), src, tgt, store)
	decls, err := rt.Execute(ctx, "crucible.js", scriptContent)
	if err != nil {
		t.Fatal(err)
	}

	if len(decls) != 1 {
		t.Fatalf("expected 1 declaration, got %d", len(decls))
	}
	// Content should be the runtime OS name
	if len(decls[0].Content) == 0 {
		t.Error("expected non-empty OS name content")
	}
}

func TestRuntime_Interrupt(t *testing.T) {
	t.Parallel()

	src := t.TempDir()
	tgt := t.TempDir()

	// Infinite loop script
	scriptContent := []byte(`while(true) {}`)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	store := fact.NewStore()
	rt := NewRuntime(ctx, slog.New(slog.DiscardHandler), src, tgt, store)
	_, err := rt.Execute(ctx, "crucible.js", scriptContent)
	if err == nil {
		t.Fatal("expected error from interrupted script")
	}

	var se *ScriptError
	if !errors.As(err, &se) {
		t.Fatalf("expected ScriptError, got %T: %v", err, err)
	}
}

func TestRuntime_ScriptError(t *testing.T) {
	t.Parallel()

	src := t.TempDir()
	tgt := t.TempDir()

	scriptContent := []byte(`throw new Error("something failed");`)

	ctx := context.Background()
	store := fact.NewStore()
	rt := NewRuntime(ctx, slog.New(slog.DiscardHandler), src, tgt, store)
	_, err := rt.Execute(ctx, "crucible.js", scriptContent)
	if err == nil {
		t.Fatal("expected error")
	}

	var se *ScriptError
	if !errors.As(err, &se) {
		t.Fatalf("expected ScriptError, got %T: %v", err, err)
	}
	if se.File != "crucible.js" {
		t.Errorf("file = %q, want crucible.js", se.File)
	}
}

func TestRuntime_ResolveContent_SourceFile(t *testing.T) {
	t.Parallel()

	src := t.TempDir()
	tgt := t.TempDir()

	if err := os.MkdirAll(filepath.Join(src, "fish"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "fish", "config.fish"), []byte("set -x PATH /usr/local/bin"), 0o644); err != nil {
		t.Fatal(err)
	}

	scriptContent := []byte(`
		var c = require("crucible");
		c.file("~/.config/fish/config.fish", { source: "fish/config.fish" });
	`)

	ctx := context.Background()
	store := fact.NewStore()
	rt := NewRuntime(ctx, slog.New(slog.DiscardHandler), src, tgt, store)
	_, err := rt.Execute(ctx, "crucible.js", scriptContent)
	if err != nil {
		t.Fatal(err)
	}

	if err := rt.ResolveContent(ctx, store); err != nil {
		t.Fatal(err)
	}

	decls := rt.Declarations()
	if string(decls[0].Content) != "set -x PATH /usr/local/bin" {
		t.Errorf("content = %q", decls[0].Content)
	}
}

func TestRuntime_ResolveContent_Template(t *testing.T) {
	t.Parallel()

	src := t.TempDir()
	tgt := t.TempDir()

	if err := os.WriteFile(filepath.Join(src, "greeting.tmpl"), []byte("Hello, {{.name}}!"), 0o644); err != nil {
		t.Fatal(err)
	}

	scriptContent := []byte(`
		var c = require("crucible");
		c.file("~/greeting.txt", { template: "greeting.tmpl", data: { name: "World" } });
	`)

	ctx := context.Background()
	store := fact.NewStore()
	rt := NewRuntime(ctx, slog.New(slog.DiscardHandler), src, tgt, store)
	_, err := rt.Execute(ctx, "crucible.js", scriptContent)
	if err != nil {
		t.Fatal(err)
	}

	if err := rt.ResolveContent(ctx, store); err != nil {
		t.Fatal(err)
	}

	decls := rt.Declarations()
	if string(decls[0].Content) != "Hello, World!" {
		t.Errorf("content = %q, want 'Hello, World!'", decls[0].Content)
	}
}

func TestRuntime_RequireRelative(t *testing.T) {
	t.Parallel()

	src := t.TempDir()
	tgt := t.TempDir()

	// Create a sub-module
	if err := os.WriteFile(filepath.Join(src, "work.js"), []byte(`
		var c = require("crucible");
		c.file("~/work.txt", { content: "work stuff" });
	`), 0o644); err != nil {
		t.Fatal(err)
	}

	scriptContent := []byte(`
		var c = require("crucible");
		c.file("~/main.txt", { content: "main stuff" });
		require("./work");
	`)
	if err := os.WriteFile(filepath.Join(src, "crucible.js"), scriptContent, 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	store := fact.NewStore()
	rt := NewRuntime(ctx, slog.New(slog.DiscardHandler), src, tgt, store)
	decls, err := rt.Execute(ctx, filepath.Join(src, "crucible.js"), scriptContent)
	if err != nil {
		t.Fatal(err)
	}

	if len(decls) != 2 {
		t.Fatalf("expected 2 declarations, got %d", len(decls))
	}
}
