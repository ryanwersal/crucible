package cli

import (
	"bytes"
	"testing"

	"github.com/ryanwersal/crucible/internal/action"
)

func TestPrintResult_GroupedTree(t *testing.T) {
	t.Parallel()

	result := action.PlanResult{
		Observations: []action.Observation{
			{Group: "Package", Description: "ripgrep (installed)"},
			{Group: "File", Description: "~/.bashrc (up to date)"},
			{Group: "Package", Description: "fd (installed)"},
		},
		Actions: []action.Action{
			{Group: "Package", Type: action.InstallPackage, Description: "install bat"},
			{Group: "File", Type: action.WriteFile, Description: "write ~/.config/fish/config.fish"},
		},
	}

	var buf bytes.Buffer
	printResult(&buf, result)
	got := buf.String()

	// Groups should appear in declaration order: package first, then file.
	wantLines := []string{
		"  package\n",
		"    ✓ ripgrep (installed)\n",
		"    ✓ fd (installed)\n",
		"    → install bat\n",
		"  file\n",
		"    ✓ ~/.bashrc (up to date)\n",
		"    → write ~/.config/fish/config.fish\n",
	}

	expected := ""
	for _, l := range wantLines {
		expected += l
	}
	if got != expected {
		t.Errorf("got:\n%s\nwant:\n%s", got, expected)
	}
}

func TestPrintResult_ActionsOnly(t *testing.T) {
	t.Parallel()

	result := action.PlanResult{
		Actions: []action.Action{
			{Group: "Display", Type: action.SetDisplay, Description: "set display density: sidebar icons → small"},
			{Group: "Package", Type: action.InstallPackage, Description: "install bat", NeedsSudo: true},
		},
	}

	var buf bytes.Buffer
	printResult(&buf, result)
	got := buf.String()

	want := "  display\n    → set display density: sidebar icons → small\n  package\n    → [sudo] install bat\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestPrintResult_EmptyGroup(t *testing.T) {
	t.Parallel()

	result := action.PlanResult{
		Observations: []action.Observation{
			{Group: "", Description: "unknown thing"},
		},
	}

	var buf bytes.Buffer
	printResult(&buf, result)
	got := buf.String()

	want := "  other\n    ✓ unknown thing\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestPrintActions_Grouped(t *testing.T) {
	t.Parallel()

	actions := []action.Action{
		{Group: "File", Type: action.WriteFile, Description: "write ~/.bashrc"},
		{Group: "Package", Type: action.InstallPackage, Description: "install ripgrep"},
		{Group: "File", Type: action.CreateSymlink, Description: "symlink ~/.vimrc"},
	}

	var buf bytes.Buffer
	printActions(&buf, actions)
	got := buf.String()

	want := "  file\n    → write ~/.bashrc\n    → symlink ~/.vimrc\n  package\n    → install ripgrep\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestGroupName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"Package", "package"},
		{"File", "file"},
		{"MiseTool", "misetool"},
		{"", "other"},
	}
	for _, tt := range tests {
		if got := groupName(tt.input); got != tt.want {
			t.Errorf("groupName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
