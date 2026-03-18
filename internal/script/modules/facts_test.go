package modules

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dop251/goja"

	"github.com/ryanwersal/crucible/internal/fact"
)

func setupFactsModule(t *testing.T) (*goja.Runtime, *fact.Store) {
	t.Helper()
	vm := goja.New()
	store := fact.NewStore()
	ctx := context.Background()

	// Pre-collect OS facts so they're cached
	_, _ = fact.Get(ctx, store, "os", fact.OSCollector{})

	mod := NewFactsModule(vm, ctx, store)
	_ = vm.Set("facts", mod.Export())
	return vm, store
}

func TestFacts_OS(t *testing.T) {
	t.Parallel()
	vm, _ := setupFactsModule(t)

	v, err := vm.RunString(`facts.os.name`)
	if err != nil {
		t.Fatal(err)
	}
	if v.String() != runtime.GOOS {
		t.Errorf("os.name = %q, want %q", v.String(), runtime.GOOS)
	}

	v, err = vm.RunString(`facts.os.arch`)
	if err != nil {
		t.Fatal(err)
	}
	if v.String() != runtime.GOARCH {
		t.Errorf("os.arch = %q, want %q", v.String(), runtime.GOARCH)
	}
}

func TestFacts_File(t *testing.T) {
	t.Parallel()
	vm, _ := setupFactsModule(t)

	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	_ = vm.Set("testPath", testFile)
	v, err := vm.RunString(`var f = facts.file(testPath); f.exists`)
	if err != nil {
		t.Fatal(err)
	}
	if !v.ToBoolean() {
		t.Error("expected file to exist")
	}
}

func TestFacts_File_NotExist(t *testing.T) {
	t.Parallel()
	vm, _ := setupFactsModule(t)

	_ = vm.Set("testPath", "/nonexistent/path/file.txt")
	v, err := vm.RunString(`facts.file(testPath).exists`)
	if err != nil {
		t.Fatal(err)
	}
	if v.ToBoolean() {
		t.Error("expected file not to exist")
	}
}

func TestFacts_Dir(t *testing.T) {
	t.Parallel()
	vm, _ := setupFactsModule(t)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "child.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	_ = vm.Set("testPath", dir)
	v, err := vm.RunString(`var d = facts.dir(testPath); d.exists`)
	if err != nil {
		t.Fatal(err)
	}
	if !v.ToBoolean() {
		t.Error("expected dir to exist")
	}
}

func TestFacts_Homebrew(t *testing.T) {
	t.Parallel()
	vm, _ := setupFactsModule(t)

	// Just verify the property exists and is a boolean (don't depend on brew being installed)
	v, err := vm.RunString(`typeof facts.homebrew.available`)
	if err != nil {
		t.Fatal(err)
	}
	if v.String() != "boolean" {
		t.Errorf("homebrew.available type = %q, want boolean", v.String())
	}
}
