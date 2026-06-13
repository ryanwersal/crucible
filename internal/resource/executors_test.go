package resource

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/ryanwersal/crucible/internal/action"
)

// These exercise the live action executors registered in the default registry
// (the path Apply actually runs), not the inert action descriptions.

func TestWriteFileExecutor(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	err := WriteFileExecutor{}.Execute(context.Background(), action.Action{
		Path:    path,
		Content: []byte("hello world"),
		Mode:    0o644,
	}, nil, io.Discard, io.Discard)
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
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o644 {
		t.Fatalf("expected mode 0644, got %04o", info.Mode().Perm())
	}
}

func TestCreateDirExecutor(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "dir")

	err := CreateDirExecutor{}.Execute(context.Background(), action.Action{
		Path: path,
		Mode: 0o755,
	}, nil, io.Discard, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		t.Fatalf("expected directory at %s (err=%v)", path, err)
	}
}

func TestCreateSymlinkExecutor(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	link := filepath.Join(dir, "link.txt")
	if err := os.WriteFile(target, []byte("target"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := CreateSymlinkExecutor{}.Execute(context.Background(), action.Action{
		Path:       link,
		LinkTarget: target,
	}, nil, io.Discard, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := os.Readlink(link); err != nil || got != target {
		t.Fatalf("expected link → %q, got %q (err=%v)", target, got, err)
	}
}

// TestCreateSymlinkExecutor_CreatesParentDirs guards the fix that lets a symlink
// be created under a tree that does not exist yet — e.g. an app support
// directory for an app that has never been launched.
func TestCreateSymlinkExecutor_CreatesParentDirs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	link := filepath.Join(dir, "Application Support", "App", "User", "settings.json")
	if err := os.WriteFile(target, []byte("target"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := CreateSymlinkExecutor{}.Execute(context.Background(), action.Action{
		Path:       link,
		LinkTarget: target,
	}, nil, io.Discard, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := os.Readlink(link); err != nil || got != target {
		t.Fatalf("expected link → %q, got %q (err=%v)", target, got, err)
	}
}

func TestSetPermissionsExecutor(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := SetPermissionsExecutor{}.Execute(context.Background(), action.Action{
		Path: path,
		Mode: 0o755,
	}, nil, io.Discard, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o755 {
		t.Fatalf("expected 0755, got %04o", info.Mode().Perm())
	}
}

func TestDeletePathExecutor(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := DeletePathExecutor{}.Execute(context.Background(), action.Action{
		Path: path,
	}, nil, io.Discard, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("expected file to be deleted")
	}
}

func TestDeletePathExecutor_Recursive(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "mydir")
	if err := os.MkdirAll(filepath.Join(path, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "sub", "file.txt"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := DeletePathExecutor{}.Execute(context.Background(), action.Action{
		Path:      path,
		Recursive: true,
	}, nil, io.Discard, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("expected directory to be deleted")
	}
}
