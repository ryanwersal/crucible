package action

import (
	"testing"

	"github.com/ryanwersal/crucible/internal/fact"
)

func TestDiffHomebrew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		desired     []DesiredPackage
		actual      *fact.HomebrewInfo
		wantActions int
		wantErr     bool
	}{
		{
			name:    "brew not available",
			desired: []DesiredPackage{{Name: "git", Type: "formula"}},
			actual:  &fact.HomebrewInfo{Available: false},
			wantErr: true,
		},
		{
			name:    "already installed",
			desired: []DesiredPackage{{Name: "git", Type: "formula"}},
			actual: &fact.HomebrewInfo{
				Available: true,
				Formulae:  map[string]bool{"git": true},
				Casks:     map[string]bool{},
			},
			wantActions: 0,
		},
		{
			name: "needs install",
			desired: []DesiredPackage{
				{Name: "git", Type: "formula"},
				{Name: "firefox", Type: "cask"},
			},
			actual: &fact.HomebrewInfo{
				Available: true,
				Formulae:  map[string]bool{"git": true},
				Casks:     map[string]bool{},
			},
			wantActions: 1,
		},
		{
			name:    "invalid package type",
			desired: []DesiredPackage{{Name: "foo", Type: "invalid"}},
			actual: &fact.HomebrewInfo{
				Available: true,
				Formulae:  map[string]bool{},
				Casks:     map[string]bool{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actions, err := DiffHomebrew(tt.desired, tt.actual)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(actions) != tt.wantActions {
				t.Fatalf("expected %d actions, got %d", tt.wantActions, len(actions))
			}
		})
	}
}
