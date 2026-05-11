package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

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

func TestPrintResult_DestructiveAction(t *testing.T) {
	t.Parallel()

	result := action.PlanResult{
		Actions: []action.Action{
			{
				Group:             "Symlink",
				Type:              action.DeletePath,
				Description:       "remove regular file /home/u/.zshrc",
				Destructive:       true,
				DestructiveReason: "regular file at /home/u/.zshrc would be deleted",
			},
			{
				Group:       "Symlink",
				Type:        action.CreateSymlink,
				Description: "create symlink /home/u/.zshrc → /home/u/dotfiles/zshrc",
			},
		},
	}

	var buf bytes.Buffer
	printResult(&buf, result)
	got := buf.String()

	if !strings.Contains(got, "⚠ remove regular file /home/u/.zshrc") {
		t.Errorf("expected destructive marker on remove action; got:\n%s", got)
	}
	if !strings.Contains(got, "destructive: regular file at /home/u/.zshrc would be deleted") {
		t.Errorf("expected destructive reason line; got:\n%s", got)
	}
	if !strings.Contains(got, "→ create symlink /home/u/.zshrc → /home/u/dotfiles/zshrc") {
		t.Errorf("non-destructive action should keep → marker; got:\n%s", got)
	}
}

func TestPlanResult_Destructive(t *testing.T) {
	t.Parallel()
	result := action.PlanResult{
		Actions: []action.Action{
			{Description: "normal 1"},
			{Description: "boom", Destructive: true, DestructiveReason: "user file"},
			{Description: "normal 2"},
		},
	}
	got := result.Destructive()
	if len(got) != 1 {
		t.Fatalf("expected 1 destructive action, got %d", len(got))
	}
	if got[0].Description != "boom" {
		t.Errorf("wrong destructive action filtered: %q", got[0].Description)
	}
}

func TestReadConfirmation_Accepts(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		input  string
		accept []string
		want   bool
	}{
		{"y matches", "y\n", []string{"y", "yes"}, true},
		{"yes matches", "yes\n", []string{"y", "yes"}, true},
		{"upper YES matches", "YES\n", []string{"y", "yes"}, true},
		{"padded whitespace matches", "  yes  \n", []string{"y", "yes"}, true},
		{"strict yes rejects y", "y\n", []string{"yes"}, false},
		{"empty rejects", "\n", []string{"y", "yes"}, false},
		{"random rejects", "maybe\n", []string{"y", "yes"}, false},
		{"EOF rejects", "", []string{"y", "yes"}, false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			var stderr bytes.Buffer
			ok, err := readConfirmation(ctx, strings.NewReader(tt.input), &stderr, tt.accept)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ok != tt.want {
				t.Errorf("got %v, want %v", ok, tt.want)
			}
		})
	}
}

func TestReadConfirmation_ContextCancel(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// blockingReader never returns — we rely on ctx to release us.
	var stderr bytes.Buffer
	ok, err := readConfirmation(ctx, blockingReader{}, &stderr, []string{"yes"})
	if err == nil {
		t.Fatal("expected context error")
	}
	if ok {
		t.Error("ok should be false when context cancels")
	}
	if !strings.Contains(stderr.String(), "Aborted") {
		t.Errorf("expected Aborted message; got %q", stderr.String())
	}
}

// blockingReader is an io.Reader whose Read blocks forever. Used to exercise
// the ctx-cancellation branch of readConfirmation.
type blockingReader struct{}

func (blockingReader) Read(_ []byte) (int, error) {
	select {} // block until goroutine is collected
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
