package action

import (
	"testing"

	"github.com/ryanwersal/crucible/internal/fact"
)

func TestDiffDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		desired     DesiredDefault
		actual      *fact.DefaultsInfo
		wantActions int
	}{
		{
			name:        "key does not exist",
			desired:     DesiredDefault{Domain: "com.apple.dock", Key: "autohide", Value: true},
			actual:      &fact.DefaultsInfo{Exists: false},
			wantActions: 1,
		},
		{
			name:        "nil actual",
			desired:     DesiredDefault{Domain: "com.apple.dock", Key: "autohide", Value: true},
			actual:      nil,
			wantActions: 1,
		},
		{
			name:        "bool matches",
			desired:     DesiredDefault{Domain: "com.apple.dock", Key: "autohide", Value: true},
			actual:      &fact.DefaultsInfo{Exists: true, Value: true},
			wantActions: 0,
		},
		{
			name:        "bool differs",
			desired:     DesiredDefault{Domain: "com.apple.dock", Key: "autohide", Value: true},
			actual:      &fact.DefaultsInfo{Exists: true, Value: false},
			wantActions: 1,
		},
		{
			name:        "int matches",
			desired:     DesiredDefault{Domain: "com.apple.dock", Key: "tilesize", Value: int64(36)},
			actual:      &fact.DefaultsInfo{Exists: true, Value: int64(36)},
			wantActions: 0,
		},
		{
			name:        "int differs",
			desired:     DesiredDefault{Domain: "com.apple.dock", Key: "tilesize", Value: int64(36)},
			actual:      &fact.DefaultsInfo{Exists: true, Value: int64(48)},
			wantActions: 1,
		},
		{
			name:        "int vs float equal",
			desired:     DesiredDefault{Domain: "com.apple.dock", Key: "tilesize", Value: int64(36)},
			actual:      &fact.DefaultsInfo{Exists: true, Value: float64(36)},
			wantActions: 0,
		},
		{
			name:        "string matches",
			desired:     DesiredDefault{Domain: "com.apple.finder", Key: "FXPreferredViewStyle", Value: "Nlsv"},
			actual:      &fact.DefaultsInfo{Exists: true, Value: "Nlsv"},
			wantActions: 0,
		},
		{
			name:        "string differs",
			desired:     DesiredDefault{Domain: "com.apple.finder", Key: "FXPreferredViewStyle", Value: "Nlsv"},
			actual:      &fact.DefaultsInfo{Exists: true, Value: "icnv"},
			wantActions: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actions := DiffDefaults(tt.desired, tt.actual)
			if len(actions) != tt.wantActions {
				t.Fatalf("expected %d actions, got %d: %v", tt.wantActions, len(actions), actions)
			}
			if tt.wantActions > 0 && actions[0].Type != SetDefaults {
				t.Fatalf("expected SetDefaults, got %s", actions[0].Type)
			}
		})
	}
}
