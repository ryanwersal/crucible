package reference_test

import (
	"testing"

	"github.com/ryanwersal/crucible/internal/reference"
	"github.com/ryanwersal/crucible/internal/resource"
	"github.com/ryanwersal/crucible/internal/script"
	"github.com/spf13/cobra"
)

func buildTestRoot() *cobra.Command {
	cmd := &cobra.Command{Use: "crucible", Short: "test root"}
	cmd.AddCommand(&cobra.Command{Use: "apply", Short: "Apply configuration"})
	cmd.AddCommand(&cobra.Command{Use: "reference", Short: "Print reference"})
	cmd.AddCommand(&cobra.Command{Use: "version", Short: "Print version"})
	return cmd
}

func TestBuildContainsAllDeclTypes(t *testing.T) {
	reg := resource.DefaultRegistry()
	declTypes := reg.AllDeclTypes()
	if len(declTypes) == 0 {
		t.Fatal("no decl types registered")
	}
	output := reference.Build(buildTestRoot(), reg)
	for _, dt := range declTypes {
		name := dt.String()
		if !containsString(output, name) {
			t.Errorf("declaration type %q not found in reference output", name)
		}
	}
}

func TestBuildContainsAllActionTypes(t *testing.T) {
	reg := resource.DefaultRegistry()
	actionTypes := reg.AllActionTypes()
	if len(actionTypes) == 0 {
		t.Fatal("no action types registered")
	}
	output := reference.Build(buildTestRoot(), reg)
	for _, at := range actionTypes {
		name := at.String()
		if !containsString(output, name) {
			t.Errorf("action type %q not found in reference output", name)
		}
	}
}

func TestBuildContainsAllTemplateFuncs(t *testing.T) {
	reg := resource.DefaultRegistry()
	output := reference.Build(buildTestRoot(), reg)
	for _, name := range script.TemplateFuncNames() {
		if !containsString(output, name) {
			t.Errorf("template function %q not found in reference output", name)
		}
	}
}

func TestBuildContainsAllJSAPIFunctions(t *testing.T) {
	reg := resource.DefaultRegistry()
	output := reference.Build(buildTestRoot(), reg)
	jsFuncs := []string{
		"file", "dir", "symlink", "brew", "defaults", "dock",
		"git", "font", "mas", "mise", "shell", "log",
	}
	for _, name := range jsFuncs {
		target := "c." + name + "("
		if !containsString(output, target) {
			t.Errorf("JS API function %q (looked for %q) not found in reference output", name, target)
		}
	}
}

func TestBuildContainsCLICommands(t *testing.T) {
	reg := resource.DefaultRegistry()
	output := reference.Build(buildTestRoot(), reg)
	for _, name := range []string{"apply", "reference", "version"} {
		if !containsString(output, name) {
			t.Errorf("CLI command %q not found in reference output", name)
		}
	}
}

func containsString(haystack, needle string) bool {
	return len(haystack) > 0 && len(needle) > 0 &&
		// simple substring search
		indexString(haystack, needle) >= 0
}

func indexString(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
