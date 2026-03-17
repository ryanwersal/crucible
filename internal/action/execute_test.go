package action

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestExecute_WriteFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	err := Execute(context.Background(), Action{
		Type:    WriteFile,
		Path:    path,
		Content: []byte("hello world"),
		Mode:    0o644,
	}, io.Discard, io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "hello world" {
		t.Fatalf("expected 'hello world', got %q", content)
	}

	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("expected mode 0644, got %04o", info.Mode().Perm())
	}
}

func TestExecute_CreateDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "dir")

	err := Execute(context.Background(), Action{
		Type: CreateDir,
		Path: path,
		Mode: 0o755,
	}, io.Discard, io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Fatal("expected directory")
	}
}

func TestExecute_CreateSymlink(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	link := filepath.Join(dir, "link.txt")

	os.WriteFile(target, []byte("target"), 0o644)

	err := Execute(context.Background(), Action{
		Type:       CreateSymlink,
		Path:       link,
		LinkTarget: target,
	}, io.Discard, io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	got, err := os.Readlink(link)
	if err != nil {
		t.Fatal(err)
	}
	if got != target {
		t.Fatalf("expected target %q, got %q", target, got)
	}
}

func TestExecute_SetPermissions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("test"), 0o644)

	err := Execute(context.Background(), Action{
		Type: SetPermissions,
		Path: path,
		Mode: 0o755,
	}, io.Discard, io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("expected 0755, got %04o", info.Mode().Perm())
	}
}

func TestExecute_DeletePath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("test"), 0o644)

	err := Execute(context.Background(), Action{
		Type: DeletePath,
		Path: path,
	}, io.Discard, io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("expected file to be deleted")
	}
}
