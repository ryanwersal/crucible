package fact

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFontCollector_Collect(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	for _, f := range []struct{ name, content string }{
		{"Mono.ttf", "fake"}, {"Sans.otf", "fake"}, {"readme.txt", "not a font"},
	} {
		if err := os.WriteFile(filepath.Join(dir, f.name), []byte(f.content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	c := FontCollector{Dir: dir}
	info, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(info.Installed) != 2 {
		t.Fatalf("expected 2 fonts, got %d: %v", len(info.Installed), info.Installed)
	}
	if !info.Installed["Mono.ttf"] {
		t.Error("expected Mono.ttf")
	}
	if !info.Installed["Sans.otf"] {
		t.Error("expected Sans.otf")
	}
}

func TestFontCollector_EmptyDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	c := FontCollector{Dir: dir}
	info, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Installed) != 0 {
		t.Fatalf("expected 0 fonts, got %d", len(info.Installed))
	}
}

func TestFontCollector_NonexistentDir(t *testing.T) {
	t.Parallel()

	c := FontCollector{Dir: "/nonexistent/fonts/dir"}
	info, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Installed) != 0 {
		t.Fatalf("expected 0 fonts, got %d", len(info.Installed))
	}
}

func TestFontCollector_AllExtensions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	for _, name := range []string{"a.ttf", "b.otf", "c.ttc", "d.woff", "e.woff2", "f.TTF"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("fake"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	c := FontCollector{Dir: dir}
	info, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(info.Installed) != 6 {
		t.Fatalf("expected 6 fonts, got %d: %v", len(info.Installed), info.Installed)
	}
}
