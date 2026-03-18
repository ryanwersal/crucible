package action

import (
	"testing"

	"github.com/ryanwersal/crucible/internal/fact"
)

func TestDiffShell(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		desired     DesiredShell
		actual      *fact.ShellInfo
		wantActions int
	}{
		{
			name:        "shell matches",
			desired:     DesiredShell{Path: "/opt/homebrew/bin/zsh", Username: "ryan"},
			actual:      &fact.ShellInfo{Path: "/opt/homebrew/bin/zsh"},
			wantActions: 0,
		},
		{
			name:        "shell differs",
			desired:     DesiredShell{Path: "/opt/homebrew/bin/zsh", Username: "ryan"},
			actual:      &fact.ShellInfo{Path: "/bin/bash"},
			wantActions: 1,
		},
		{
			name:        "nil actual",
			desired:     DesiredShell{Path: "/opt/homebrew/bin/zsh", Username: "ryan"},
			actual:      nil,
			wantActions: 1,
		},
		{
			name:        "empty actual",
			desired:     DesiredShell{Path: "/opt/homebrew/bin/zsh", Username: "ryan"},
			actual:      &fact.ShellInfo{Path: ""},
			wantActions: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actions := DiffShell(tt.desired, tt.actual)
			if len(actions) != tt.wantActions {
				t.Fatalf("expected %d actions, got %d: %v", tt.wantActions, len(actions), actions)
			}
			if tt.wantActions > 0 && actions[0].Type != SetShell {
				t.Errorf("expected SetShell, got %s", actions[0].Type)
			}
		})
	}
}
