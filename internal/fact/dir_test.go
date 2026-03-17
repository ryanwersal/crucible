package fact

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDirCollector_ExistingDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0o644)

	c := DirCollector{Path: dir}
	info, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if !info.Exists {
		t.Fatal("expected Exists=true")
	}
	if len(info.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(info.Children))
	}
}

func TestDirCollector_NonExistent(t *testing.T) {
	t.Parallel()
	c := DirCollector{Path: "/nonexistent/dir/surely"}
	info, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.Exists {
		t.Fatal("expected Exists=false")
	}
}

func TestDirCollector_FileNotDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	os.WriteFile(path, []byte("not a dir"), 0o644)

	c := DirCollector{Path: path}
	info, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.Exists {
		t.Fatal("expected Exists=false for a file path")
	}
}
