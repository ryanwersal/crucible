package fact

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"syscall"
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

	if err := os.WriteFile(target, []byte("target"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

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

func TestFileCollector_RejectsFIFO(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "fifo")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := (FileCollector{Path: path}).Collect(t.Context()); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("expected special-file conflict, got %v", err)
	}
}

func TestFileCollectorSkipHash(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "seed")
	if err := os.WriteFile(path, []byte("app state"), 0o000); err != nil {
		t.Fatal(err)
	}
	info, err := (FileCollector{Path: path, SkipHash: true}).Collect(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !info.Exists || info.Hash != "" || info.IsDir || info.IsLink {
		t.Fatalf("unexpected metadata: %+v", info)
	}
}
