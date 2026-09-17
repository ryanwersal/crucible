package resource

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/fact"
)

func TestSeedExecutorPreservesFilesCreatedAfterPlanning(t *testing.T) {
	t.Parallel()
	for _, exists := range []bool{false, true} {
		name := "missing"
		if exists {
			name = "created after planning"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			path := filepath.Join(dir, "seed")
			acts, err := action.DiffFile(action.DesiredFile{Path: path, Content: []byte("seed"), Mode: 0o644, Seed: true}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if exists {
				if err := os.WriteFile(path, []byte("app state"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := (WriteFileExecutor{}).Execute(t.Context(), acts[0], nil, io.Discard, io.Discard); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			want := "seed"
			if exists {
				want = "app state"
			}
			if string(data) != want {
				t.Fatalf("contents = %q, want %q", data, want)
			}
			if exists {
				info, err := os.Stat(path)
				if err != nil {
					t.Fatal(err)
				}
				if info.Mode().Perm() != 0o600 {
					t.Fatal("seed changed existing permissions")
				}
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 {
				t.Fatalf("left temporary files: %v", entries)
			}
		})
	}
}

func TestSeedExecutorRejectsNonregularDestination(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"directory", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			path := filepath.Join(dir, "seed")
			var err error
			if kind == "directory" {
				err = os.Mkdir(path, 0o700)
			} else {
				err = os.Symlink("missing", path)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := (WriteFileExecutor{}).Execute(t.Context(), action.Action{Path: path, Content: []byte("seed"), Mode: 0o644, Seed: true}, nil, io.Discard, io.Discard); err == nil {
				t.Fatal("expected conflict")
			}
			info, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().IsRegular() {
				t.Fatal("replaced nonregular destination")
			}
		})
	}
}

func TestSeedMigrationPreservesReplacement(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"regular", "directory", "symlink", "missing"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "seed")
			acts, err := action.DiffFile(action.DesiredFile{Path: path, Content: []byte("seed"), Mode: 0o644, Seed: true}, &fact.FileInfo{Exists: true, IsLink: true})
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "regular":
				err = os.WriteFile(path, []byte("app state"), 0o600)
			case "directory":
				err = os.Mkdir(path, 0o700)
			case "symlink":
				err = os.Symlink("missing-target", path)
			}
			if err != nil {
				t.Fatal(err)
			}
			err = (DeletePathExecutor{}).Execute(t.Context(), acts[0], nil, io.Discard, io.Discard)
			if kind == "directory" {
				if err == nil {
					t.Fatal("expected directory conflict")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := (WriteFileExecutor{}).Execute(t.Context(), acts[1], nil, io.Discard, io.Discard); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			want := "seed"
			if kind == "regular" {
				want = "app state"
			}
			if string(data) != want {
				t.Fatalf("got %q, want %q", data, want)
			}
		})
	}
}
