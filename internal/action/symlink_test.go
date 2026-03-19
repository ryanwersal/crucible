package action

import (
	"testing"

	"github.com/ryanwersal/crucible/internal/fact"
)

func TestDiffSymlink(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		desired     DesiredSymlink
		actual      *fact.SymlinkInfo
		wantActions int
		wantType    Type
	}{
		{
			name:        "new symlink",
			desired:     DesiredSymlink{Path: "/tmp/link", Target: "/tmp/target"},
			actual:      nil,
			wantActions: 1,
			wantType:    CreateSymlink,
		},
		{
			name:        "no change",
			desired:     DesiredSymlink{Path: "/tmp/link", Target: "/tmp/target"},
			actual:      &fact.SymlinkInfo{Exists: true, Target: "/tmp/target"},
			wantActions: 0,
		},
		{
			name:        "target changed",
			desired:     DesiredSymlink{Path: "/tmp/link", Target: "/tmp/new"},
			actual:      &fact.SymlinkInfo{Exists: true, Target: "/tmp/old"},
			wantActions: 2,
			wantType:    DeletePath,
		},
		{
			name:        "absent and exists",
			desired:     DesiredSymlink{Path: "/tmp/link", Absent: true},
			actual:      &fact.SymlinkInfo{Exists: true, Target: "/tmp/target"},
			wantActions: 1,
			wantType:    DeletePath,
		},
		{
			name:        "absent and does not exist",
			desired:     DesiredSymlink{Path: "/tmp/link", Absent: true},
			actual:      nil,
			wantActions: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actions := DiffSymlink(tt.desired, tt.actual)
			if len(actions) != tt.wantActions {
				t.Fatalf("expected %d actions, got %d: %v", tt.wantActions, len(actions), actions)
			}
			if tt.wantActions > 0 && actions[0].Type != tt.wantType {
				t.Fatalf("expected type %s, got %s", tt.wantType, actions[0].Type)
			}
		})
	}
}
