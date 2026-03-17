package script

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoader_EntryPoint_Found(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	content := []byte(`var x = 1;`)
	os.WriteFile(filepath.Join(dir, "crucible.js"), content, 0o644)

	loader := NewLoader(dir)
	path, got, err := loader.EntryPoint()
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Join(dir, "crucible.js") {
		t.Errorf("path = %q, want %q", path, filepath.Join(dir, "crucible.js"))
	}
	if string(got) != string(content) {
		t.Errorf("content = %q, want %q", got, content)
	}
}

func TestLoader_EntryPoint_NotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	loader := NewLoader(dir)
	_, _, err := loader.EntryPoint()
	if !errors.Is(err, ErrNoScript) {
		t.Errorf("expected ErrNoScript, got %v", err)
	}
}

func TestLoader_EntryPoint_PermissionError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	path := filepath.Join(dir, "crucible.js")
	os.WriteFile(path, []byte("x"), 0o000)
	t.Cleanup(func() { os.Chmod(path, 0o644) })

	loader := NewLoader(dir)
	_, _, err := loader.EntryPoint()
	if err == nil {
		t.Fatal("expected error for unreadable file")
	}
	if errors.Is(err, ErrNoScript) {
		t.Error("should not be ErrNoScript for permission error")
	}
}
