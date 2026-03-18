package fact

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSymlinkCollector_ExistingSymlink(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	link := filepath.Join(dir, "link")

	if err := os.WriteFile(target, []byte("t"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	c := SymlinkCollector{Path: link}
	info, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if !info.Exists {
		t.Fatal("expected Exists=true")
	}
	if info.Target != target {
		t.Fatalf("target mismatch: got %q, want %q", info.Target, target)
	}
}

func TestSymlinkCollector_NonExistent(t *testing.T) {
	t.Parallel()
	c := SymlinkCollector{Path: "/nonexistent/link"}
	info, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.Exists {
		t.Fatal("expected Exists=false")
	}
}

func TestSymlinkCollector_RegularFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "regular.txt")
	if err := os.WriteFile(path, []byte("not a link"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := SymlinkCollector{Path: path}
	info, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.Exists {
		t.Fatal("expected Exists=false for regular file")
	}
}
