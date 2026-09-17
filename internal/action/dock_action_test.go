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
		wantDesc    string
	}{
		{
			name: "nil actual",
			desired: DesiredDock{
				Layout: true,
				Apps:   []string{"/Applications/Safari.app"},
			},
			actual:      nil,
			wantActions: 1,
		},
		{
			name: "apps match",
			desired: DesiredDock{
				Layout: true,
				Apps:   []string{"/Applications/Safari.app", "/Applications/Firefox.app"},
			},
			actual: &fact.DockInfo{
				Apps: []string{"/Applications/Safari.app", "/Applications/Firefox.app"},
			},
			wantActions: 0,
		},
		{
			name: "apps differ",
			desired: DesiredDock{
				Layout: true,
				Apps:   []string{"/Applications/Safari.app"},
			},
			actual: &fact.DockInfo{
				Apps: []string{"/Applications/Firefox.app"},
			},
			wantActions: 1,
		},
		{
			name: "apps order matters",
			desired: DesiredDock{
				Layout: true,
				Apps:   []string{"/Applications/Safari.app", "/Applications/Firefox.app"},
			},
			actual: &fact.DockInfo{
				Apps: []string{"/Applications/Firefox.app", "/Applications/Safari.app"},
			},
			wantActions: 1,
		},
		{
			name: "folders match",
			desired: DesiredDock{
				Layout:  true,
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
			name: "settings only, layout untouched",
			desired: DesiredDock{
				Settings: DockSettings{Autohide: new(true), TileSize: 48},
			},
			actual: &fact.DockInfo{
				Apps:     []string{"/Applications/Firefox.app"},
				Autohide: new(true),
				TileSize: 48,
			},
			wantActions: 0,
		},
		{
			name: "autohide differs",
			desired: DesiredDock{
				Settings: DockSettings{Autohide: new(true)},
			},
			actual:      &fact.DockInfo{Autohide: new(false)},
			wantActions: 1,
		},
		{
			name: "autohide key absent",
			desired: DesiredDock{
				Settings: DockSettings{Autohide: new(false)},
			},
			actual:      &fact.DockInfo{},
			wantActions: 1,
		},
		{
			name: "tile size differs",
			desired: DesiredDock{
				Settings: DockSettings{TileSize: 48},
			},
			actual:      &fact.DockInfo{TileSize: 36},
			wantActions: 1,
			wantDesc:    "set dock: tile size → 48",
		},
		{
			name: "show recents differs",
			desired: DesiredDock{
				Settings: DockSettings{ShowRecents: new(false)},
			},
			actual:      &fact.DockInfo{ShowRecents: new(true)},
			wantActions: 1,
		},
		{
			name: "folders differ",
			desired: DesiredDock{
				Layout:  true,
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
			if len(actions) > 0 && actions[0].SerialGroup != "dock" {
				t.Fatal("Dock writes must share a serial group")
			}
			if tt.wantDesc != "" && actions[0].Description != tt.wantDesc {
				t.Fatalf("description = %q, want %q", actions[0].Description, tt.wantDesc)
			}
		})
	}
}
