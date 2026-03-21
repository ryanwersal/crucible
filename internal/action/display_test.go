package action

import (
	"testing"

	"github.com/ryanwersal/crucible/internal/fact"
)

func TestDiffDisplay(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		desired   DesiredDisplay
		actual    *fact.DisplayInfo
		wantCount int
	}{
		{
			name:    "all up to date",
			desired: DesiredDisplay{SidebarIconSize: "small", MenuBarSpacing: "compact"},
			actual: &fact.DisplayInfo{
				SidebarIconSize: 1,
				MenuBarSpacing:  6,
				MenuBarPadding:  4,
			},
			wantCount: 0,
		},
		{
			name:    "sidebar needs change",
			desired: DesiredDisplay{SidebarIconSize: "small"},
			actual: &fact.DisplayInfo{
				SidebarIconSize: 2,
				MenuBarSpacing:  -1,
				MenuBarPadding:  -1,
			},
			wantCount: 1,
		},
		{
			name:    "menu bar needs change to compact",
			desired: DesiredDisplay{MenuBarSpacing: "compact"},
			actual: &fact.DisplayInfo{
				MenuBarSpacing: -1,
				MenuBarPadding: -1,
			},
			wantCount: 1,
		},
		{
			name:    "menu bar needs change to default",
			desired: DesiredDisplay{MenuBarSpacing: "default"},
			actual: &fact.DisplayInfo{
				MenuBarSpacing: 6,
				MenuBarPadding: 4,
			},
			wantCount: 1,
		},
		{
			name:    "menu bar already default",
			desired: DesiredDisplay{MenuBarSpacing: "default"},
			actual: &fact.DisplayInfo{
				MenuBarSpacing: -1,
				MenuBarPadding: -1,
			},
			wantCount: 0,
		},
		{
			name:    "resolution needs change",
			desired: DesiredDisplay{Resolution: "1800x1169"},
			actual: &fact.DisplayInfo{
				Resolution:     "1512x982",
				MenuBarSpacing: -1,
				MenuBarPadding: -1,
			},
			wantCount: 1,
		},
		{
			name:    "resolution with hz needs change",
			desired: DesiredDisplay{Resolution: "1800x1169", HZ: 120},
			actual: &fact.DisplayInfo{
				Resolution:     "1800x1169",
				HZ:             60,
				MenuBarSpacing: -1,
				MenuBarPadding: -1,
			},
			wantCount: 1,
		},
		{
			name:    "resolution and hz match",
			desired: DesiredDisplay{Resolution: "1800x1169", HZ: 120},
			actual: &fact.DisplayInfo{
				Resolution:     "1800x1169",
				HZ:             120,
				MenuBarSpacing: -1,
				MenuBarPadding: -1,
			},
			wantCount: 0,
		},
		{
			name:    "resolution without hz ignores hz",
			desired: DesiredDisplay{Resolution: "1800x1169"},
			actual: &fact.DisplayInfo{
				Resolution:     "1800x1169",
				HZ:             60,
				MenuBarSpacing: -1,
				MenuBarPadding: -1,
			},
			wantCount: 0,
		},
		{
			name:      "nil actual",
			desired:   DesiredDisplay{SidebarIconSize: "small"},
			actual:    nil,
			wantCount: 1,
		},
		{
			name:    "nothing specified, nothing to do",
			desired: DesiredDisplay{},
			actual: &fact.DisplayInfo{
				MenuBarSpacing: -1,
				MenuBarPadding: -1,
			},
			wantCount: 0,
		},
		{
			name:    "multiple changes bundled into one action",
			desired: DesiredDisplay{SidebarIconSize: "small", MenuBarSpacing: "compact", Resolution: "1800x1169"},
			actual: &fact.DisplayInfo{
				SidebarIconSize: 3,
				MenuBarSpacing:  -1,
				MenuBarPadding:  -1,
				Resolution:      "1512x982",
			},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actions := DiffDisplay(tt.desired, tt.actual)
			if len(actions) != tt.wantCount {
				t.Fatalf("expected %d actions, got %d: %v", tt.wantCount, len(actions), actions)
			}
			if tt.wantCount > 0 && actions[0].Type != SetDisplay {
				t.Errorf("expected SetDisplay, got %s", actions[0].Type)
			}
		})
	}
}
