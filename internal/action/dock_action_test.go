package action

import (
	"testing"

	"github.com/ryanwersal/crucible/internal/fact"
)

func TestDiffDock(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		desired     DesiredDock
		actual      *fact.DockInfo
		wantActions int
	}{
		{
			name: "nil actual",
			desired: DesiredDock{
				Apps: []string{"/Applications/Safari.app"},
			},
			actual:      nil,
			wantActions: 1,
		},
		{
			name: "apps match",
			desired: DesiredDock{
				Apps: []string{"/Applications/Safari.app", "/Applications/Firefox.app"},
			},
			actual: &fact.DockInfo{
				Apps: []string{"/Applications/Safari.app", "/Applications/Firefox.app"},
			},
			wantActions: 0,
		},
		{
			name: "apps differ",
			desired: DesiredDock{
				Apps: []string{"/Applications/Safari.app"},
			},
			actual: &fact.DockInfo{
				Apps: []string{"/Applications/Firefox.app"},
			},
			wantActions: 1,
		},
		{
			name: "apps order matters",
			desired: DesiredDock{
				Apps: []string{"/Applications/Safari.app", "/Applications/Firefox.app"},
			},
			actual: &fact.DockInfo{
				Apps: []string{"/Applications/Firefox.app", "/Applications/Safari.app"},
			},
			wantActions: 1,
		},
		{
			name: "folders match",
			desired: DesiredDock{
				Apps:    []string{"/Applications/Safari.app"},
				Folders: []DockFolder{{Path: "/Users/test/Downloads", View: "grid", Display: "folder"}},
			},
			actual: &fact.DockInfo{
				Apps:    []string{"/Applications/Safari.app"},
				Folders: []fact.DockFolderInfo{{Path: "/Users/test/Downloads", View: "grid", Display: "folder"}},
			},
			wantActions: 0,
		},
		{
			name: "folders differ",
			desired: DesiredDock{
				Apps:    []string{"/Applications/Safari.app"},
				Folders: []DockFolder{{Path: "/Users/test/Downloads", View: "grid", Display: "folder"}},
			},
			actual: &fact.DockInfo{
				Apps:    []string{"/Applications/Safari.app"},
				Folders: []fact.DockFolderInfo{{Path: "/Users/test/Downloads", View: "list", Display: "folder"}},
			},
			wantActions: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actions := DiffDock(tt.desired, tt.actual)
			if len(actions) != tt.wantActions {
				t.Fatalf("expected %d actions, got %d: %v", tt.wantActions, len(actions), actions)
			}
			if tt.wantActions > 0 && actions[0].Type != SetDock {
				t.Fatalf("expected SetDock, got %s", actions[0].Type)
			}
		})
	}
}
