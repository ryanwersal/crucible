package fact

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestFileCollector_ExistingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := []byte("hello world\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	c := FileCollector{Path: path}
	info, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if !info.Exists {
		t.Fatal("expected Exists=true")
	}
	if info.IsDir {
		t.Fatal("expected IsDir=false")
	}
	if info.IsLink {
		t.Fatal("expected IsLink=false")
	}

	h := sha256.Sum256(content)
	expected := hex.EncodeToString(h[:])
	if info.Hash != expected {
		t.Fatalf("hash mismatch: got %s, want %s", info.Hash, expected)
	}
}

func TestFileCollector_NonExistent(t *testing.T) {
	t.Parallel()
	c := FileCollector{Path: "/nonexistent/path/surely"}
	info, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.Exists {
		t.Fatal("expected Exists=false")
	}
}

func TestFileCollector_Symlink(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	link := filepath.Join(dir, "link.txt")

	os.WriteFile(target, []byte("target"), 0o644)
	os.Symlink(target, link)

	c := FileCollector{Path: link}
	info, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if !info.Exists {
		t.Fatal("expected Exists=true")
	}
	if !info.IsLink {
		t.Fatal("expected IsLink=true")
	}
}
