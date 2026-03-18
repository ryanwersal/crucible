package action

import (
	"testing"

	"github.com/ryanwersal/crucible/internal/fact"
)

func TestDiffFonts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		desired     []DesiredFont
		actual      *fact.FontInfo
		wantActions int
	}{
		{
			name: "font not installed",
			desired: []DesiredFont{
				{Source: "/src/fonts/Mono.ttf", Name: "Mono.ttf", DestDir: "/home/user/Library/Fonts"},
			},
			actual:      &fact.FontInfo{Installed: map[string]bool{}},
			wantActions: 1,
		},
		{
			name: "font already installed",
			desired: []DesiredFont{
				{Source: "/src/fonts/Mono.ttf", Name: "Mono.ttf", DestDir: "/home/user/Library/Fonts"},
			},
			actual:      &fact.FontInfo{Installed: map[string]bool{"Mono.ttf": true}},
			wantActions: 0,
		},
		{
			name: "nil actual",
			desired: []DesiredFont{
				{Source: "/src/fonts/Mono.ttf", Name: "Mono.ttf", DestDir: "/home/user/Library/Fonts"},
			},
			actual:      nil,
			wantActions: 1,
		},
		{
			name: "mixed installed and missing",
			desired: []DesiredFont{
				{Source: "/src/fonts/Mono.ttf", Name: "Mono.ttf", DestDir: "/home/user/Library/Fonts"},
				{Source: "/src/fonts/Sans.otf", Name: "Sans.otf", DestDir: "/home/user/Library/Fonts"},
			},
			actual:      &fact.FontInfo{Installed: map[string]bool{"Mono.ttf": true}},
			wantActions: 1,
		},
		{
			name:        "empty desired",
			desired:     nil,
			actual:      &fact.FontInfo{Installed: map[string]bool{"Mono.ttf": true}},
			wantActions: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actions := DiffFonts(tt.desired, tt.actual)
			if len(actions) != tt.wantActions {
				t.Fatalf("expected %d actions, got %d: %v", tt.wantActions, len(actions), actions)
			}
			for _, a := range actions {
				if a.Type != InstallFont {
					t.Errorf("expected InstallFont, got %s", a.Type)
				}
			}
		})
	}
}
