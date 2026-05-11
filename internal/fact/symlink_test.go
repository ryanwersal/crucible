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

	if info.Kind != PathSymlink {
		t.Errorf("Kind = %v, want PathSymlink", info.Kind)
	}
	if !info.IsSymlink() {
		t.Error("IsSymlink() should be true")
	}
	if !info.Exists() {
		t.Error("Exists() should be true")
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
	if info.Kind != PathMissing {
		t.Errorf("Kind = %v, want PathMissing", info.Kind)
	}
	if info.Exists() {
		t.Error("Exists() should be false")
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
	if info.Kind != PathRegularFile {
		t.Errorf("Kind = %v, want PathRegularFile", info.Kind)
	}
	if info.IsSymlink() {
		t.Error("IsSymlink() should be false for a regular file")
	}
	if !info.Exists() {
		t.Error("Exists() should be true (something is there)")
	}
}

func TestSymlinkCollector_Directory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}

	c := SymlinkCollector{Path: path}
	info, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.Kind != PathDirectory {
		t.Errorf("Kind = %v, want PathDirectory", info.Kind)
	}
}

func TestPathKindString(t *testing.T) {
	t.Parallel()
	cases := map[PathKind]string{
		PathMissing:     "missing",
		PathSymlink:     "symlink",
		PathRegularFile: "regular file",
		PathDirectory:   "directory",
		PathOther:       "other",
	}
	for k, want := range cases {
		if got := k.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", k, got, want)
		}
	}
}
