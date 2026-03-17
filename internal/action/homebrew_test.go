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
			desired: []DesiredPackage{{Name: "git"}},
			actual:  &fact.HomebrewInfo{Available: false},
			wantErr: true,
		},
		{
			name:    "formula already installed",
			desired: []DesiredPackage{{Name: "git"}},
			actual: &fact.HomebrewInfo{
				Available: true,
				Formulae:  map[string]bool{"git": true},
				Casks:     map[string]bool{},
			},
			wantActions: 0,
		},
		{
			name:    "cask already installed",
			desired: []DesiredPackage{{Name: "alacritty"}},
			actual: &fact.HomebrewInfo{
				Available: true,
				Formulae:  map[string]bool{},
				Casks:     map[string]bool{"alacritty": true},
			},
			wantActions: 0,
		},
		{
			name: "needs install",
			desired: []DesiredPackage{
				{Name: "git"},
				{Name: "alacritty"},
			},
			actual: &fact.HomebrewInfo{
				Available: true,
				Formulae:  map[string]bool{"git": true},
				Casks:     map[string]bool{},
			},
			wantActions: 1,
		},
		{
			name:    "tap-qualified name matches short name",
			desired: []DesiredPackage{{Name: "ryanwersal/tools/helios"}},
			actual: &fact.HomebrewInfo{
				Available: true,
				Formulae:  map[string]bool{"helios": true},
				Casks:     map[string]bool{},
			},
			wantActions: 0,
		},
		{
			name:    "tap-qualified name not installed",
			desired: []DesiredPackage{{Name: "ryanwersal/tools/helios"}},
			actual: &fact.HomebrewInfo{
				Available: true,
				Formulae:  map[string]bool{},
				Casks:     map[string]bool{},
			},
			wantActions: 1,
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

func TestShortName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"git", "git"},
		{"ryanwersal/tools/helios", "helios"},
		{"owner/tap/formula", "formula"},
	}

	for _, tt := range tests {
		if got := shortName(tt.input); got != tt.want {
			t.Errorf("shortName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
