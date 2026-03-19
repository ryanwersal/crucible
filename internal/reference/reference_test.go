package reference_test

import (
	"testing"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/reference"
	"github.com/ryanwersal/crucible/internal/script"
	"github.com/ryanwersal/crucible/internal/script/decl"
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
	output := reference.Build(buildTestRoot())
	for _, dt := range decl.AllTypes() {
		name := dt.String()
		if !containsString(output, name) {
			t.Errorf("declaration type %q not found in reference output", name)
		}
	}
}

func TestBuildContainsAllActionTypes(t *testing.T) {
	output := reference.Build(buildTestRoot())
	for _, at := range action.AllTypes() {
		name := at.String()
		if !containsString(output, name) {
			t.Errorf("action type %q not found in reference output", name)
		}
	}
}

func TestBuildContainsAllTemplateFuncs(t *testing.T) {
	output := reference.Build(buildTestRoot())
	for _, name := range script.TemplateFuncNames() {
		if !containsString(output, name) {
			t.Errorf("template function %q not found in reference output", name)
		}
	}
}

func TestBuildContainsAllJSAPIFunctions(t *testing.T) {
	output := reference.Build(buildTestRoot())
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
	output := reference.Build(buildTestRoot())
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
