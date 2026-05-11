package modules

import (
	"strings"
	"testing"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
)

func setupPathVM(t *testing.T) *goja.Runtime {
	t.Helper()
	vm := goja.New()
	registry := require.NewRegistry()
	registry.Enable(vm)
	RegisterPath(registry)
	return vm
}

func TestPathJoin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		js   string
		want string
	}{
		{
			name: "node:path specifier",
			js:   `require("node:path").join("a", "b", "c")`,
			want: "a/b/c",
		},
		{
			name: "bare path specifier",
			js:   `require("path").join("a", "b", "c")`,
			want: "a/b/c",
		},
		{
			name: "absolute root",
			js:   `require("node:path").join("/foo", "bar", "baz/asdf", "quux", "..")`,
			want: "/foo/bar/baz/asdf",
		},
		{
			name: "no args returns dot",
			js:   `require("node:path").join()`,
			want: ".",
		},
		{
			name: "empty segments collapse",
			js:   `require("node:path").join("foo", "", "bar")`,
			want: "foo/bar",
		},
		{
			name: "duplicate separators normalized",
			js:   `require("node:path").join("a/", "/b", "c")`,
			want: "a/b/c",
		},
		{
			name: "single segment passes through",
			js:   `require("node:path").join("only")`,
			want: "only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			vm := setupPathVM(t)
			got, err := vm.RunString(tt.js)
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}
			if s := got.String(); s != tt.want {
				t.Errorf("got %q, want %q", s, tt.want)
			}
		})
	}
}

func TestPathJoin_NonStringArg(t *testing.T) {
	t.Parallel()
	vm := setupPathVM(t)
	_, err := vm.RunString(`require("node:path").join("foo", 42, "bar")`)
	if err == nil {
		t.Fatal("expected TypeError, got nil")
	}
	if !strings.Contains(err.Error(), "TypeError") {
		t.Fatalf("expected TypeError, got: %v", err)
	}
	if !strings.Contains(err.Error(), "index 1") {
		t.Fatalf("expected error to mention argument index, got: %v", err)
	}
}
