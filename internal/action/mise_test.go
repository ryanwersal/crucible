package action

import (
	"strings"
	"testing"

	"github.com/ryanwersal/crucible/internal/fact"
)

func TestDiffMise(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		desired         []DesiredMiseTool
		actual          *fact.MiseInfo
		wantActions     int
		wantErr         bool
		wantDescContain string
	}{
		{
			name:    "mise not available",
			desired: []DesiredMiseTool{{Name: "python", Version: "3.12"}},
			actual:  &fact.MiseInfo{Available: false},
			wantErr: true,
		},
		{
			name:    "nil actual",
			desired: []DesiredMiseTool{{Name: "python", Version: "3.12"}},
			actual:  nil,
			wantErr: true,
		},
		{
			name:    "tool at correct version",
			desired: []DesiredMiseTool{{Name: "python", Version: "3.12"}},
			actual: &fact.MiseInfo{
				Available: true,
				Globals:   map[string]string{"python": "3.12"},
			},
			wantActions: 0,
		},
		{
			name:    "tool needs install",
			desired: []DesiredMiseTool{{Name: "python", Version: "3.12"}},
			actual: &fact.MiseInfo{
				Available: true,
				Globals:   map[string]string{},
			},
			wantActions: 1,
		},
		{
			name:    "tool at wrong version",
			desired: []DesiredMiseTool{{Name: "python", Version: "3.12"}},
			actual: &fact.MiseInfo{
				Available: true,
				Globals:   map[string]string{"python": "3.11"},
			},
			wantActions:     1,
			wantDescContain: "3.11 → 3.12",
		},
		{
			name: "mixed installed and missing",
			desired: []DesiredMiseTool{
				{Name: "python", Version: "3.12"},
				{Name: "node", Version: "22"},
			},
			actual: &fact.MiseInfo{
				Available: true,
				Globals:   map[string]string{"python": "3.12"},
			},
			wantActions: 1,
		},
		{
			name:    "absent and installed",
			desired: []DesiredMiseTool{{Name: "python", Absent: true}},
			actual: &fact.MiseInfo{
				Available: true,
				Globals:   map[string]string{"python": "3.12"},
			},
			wantActions: 1,
		},
		{
			name:    "absent and not installed",
			desired: []DesiredMiseTool{{Name: "python", Absent: true}},
			actual: &fact.MiseInfo{
				Available: true,
				Globals:   map[string]string{},
			},
			wantActions: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actions, err := DiffMise(tt.desired, tt.actual)
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
			for _, a := range actions {
				if a.Type != InstallMiseTool && a.Type != UninstallMiseTool {
					t.Errorf("expected InstallMiseTool or UninstallMiseTool, got %s", a.Type)
				}
			}
			if tt.wantDescContain != "" && tt.wantActions > 0 {
				if !strings.Contains(actions[0].Description, tt.wantDescContain) {
					t.Errorf("description %q should contain %q", actions[0].Description, tt.wantDescContain)
				}
			}
		})
	}
}
