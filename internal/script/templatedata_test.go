package script

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/ryanwersal/crucible/internal/fact"
)

func TestFactsToTemplateData(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := fact.NewStore()
	// Pre-collect OS facts so the store has them cached
	fact.Get(ctx, store, "os", fact.OSCollector{})

	logger := slog.New(slog.DiscardHandler)
	data := factsToTemplateData(ctx, logger, store)

	osData, ok := data["os"].(map[string]any)
	if !ok {
		t.Fatal("expected os key in template data")
	}

	if osData["name"] != runtime.GOOS {
		t.Errorf("os.name = %v, want %v", osData["name"], runtime.GOOS)
	}
	if osData["arch"] != runtime.GOARCH {
		t.Errorf("os.arch = %v, want %v", osData["arch"], runtime.GOARCH)
	}
	if osData["hostname"] == nil || osData["hostname"] == "" {
		t.Error("expected non-empty hostname")
	}
}

func TestMergeTemplateData_UserOverrides(t *testing.T) {
	t.Parallel()

	base := map[string]any{
		"os": map[string]any{
			"name": "darwin",
			"arch": "arm64",
		},
		"homebrew": map[string]any{
			"available": true,
		},
	}

	user := map[string]any{
		"os":   "custom",
		"name": "myapp",
	}

	merged := mergeTemplateData(base, user)

	// User's "os" should override the auto-injected nested map
	if merged["os"] != "custom" {
		t.Errorf("os = %v, want 'custom'", merged["os"])
	}
	// User's extra keys should be present
	if merged["name"] != "myapp" {
		t.Errorf("name = %v, want 'myapp'", merged["name"])
	}
	// Base keys not overridden should remain
	if _, ok := merged["homebrew"]; !ok {
		t.Error("expected homebrew key to remain")
	}
}

func TestMergeTemplateData_EmptyUser(t *testing.T) {
	t.Parallel()

	base := map[string]any{"os": map[string]any{"name": "darwin"}}
	merged := mergeTemplateData(base, nil)

	if merged["os"] == nil {
		t.Error("expected os key in merged data")
	}
}

func TestResolveContent_TemplateWithAutoFacts(t *testing.T) {
	t.Parallel()

	src := t.TempDir()
	tgt := t.TempDir()

	// Template that uses auto-injected facts and a template function
	tmplContent := `OS: {{ .os.name }}, Arch: {{ .os.arch }}, Home: {{ env "HOME" | default "/unknown" }}`
	os.WriteFile(filepath.Join(src, "config.tmpl"), []byte(tmplContent), 0o644)

	scriptContent := []byte(`
		var c = require("crucible");
		c.file("~/config.txt", { template: "config.tmpl" });
	`)

	ctx := context.Background()
	store := fact.NewStore()
	fact.Get(ctx, store, "os", fact.OSCollector{})

	rt := NewRuntime(ctx, slog.New(slog.DiscardHandler), src, tgt, store)
	_, err := rt.Execute(ctx, "crucible.js", scriptContent)
	if err != nil {
		t.Fatal(err)
	}

	if err := rt.ResolveContent(ctx, store); err != nil {
		t.Fatal(err)
	}

	decls := rt.Declarations()
	got := string(decls[0].Content)

	expected := "OS: " + runtime.GOOS + ", Arch: " + runtime.GOARCH + ", Home: " + os.Getenv("HOME")
	if os.Getenv("HOME") == "" {
		expected = "OS: " + runtime.GOOS + ", Arch: " + runtime.GOARCH + ", Home: /unknown"
	}

	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestResolveContent_UserDataOverridesFacts(t *testing.T) {
	t.Parallel()

	src := t.TempDir()
	tgt := t.TempDir()

	tmplContent := `OS: {{ .os }}`
	os.WriteFile(filepath.Join(src, "override.tmpl"), []byte(tmplContent), 0o644)

	// User provides a flat "os" string that should override the auto-injected map
	scriptContent := []byte(`
		var c = require("crucible");
		c.file("~/override.txt", { template: "override.tmpl", data: { os: "custom-os" } });
	`)

	ctx := context.Background()
	store := fact.NewStore()
	fact.Get(ctx, store, "os", fact.OSCollector{})

	rt := NewRuntime(ctx, slog.New(slog.DiscardHandler), src, tgt, store)
	_, err := rt.Execute(ctx, "crucible.js", scriptContent)
	if err != nil {
		t.Fatal(err)
	}

	if err := rt.ResolveContent(ctx, store); err != nil {
		t.Fatal(err)
	}

	decls := rt.Declarations()
	got := string(decls[0].Content)
	if got != "OS: custom-os" {
		t.Errorf("got %q, want 'OS: custom-os'", got)
	}
}
