package action

import (
	"testing"

	"github.com/ryanwersal/crucible/internal/fact"
)

func TestDiffDir(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		desired     DesiredDir
		actual      *fact.DirInfo
		wantActions int
		wantType    Type
	}{
		{
			name:        "new directory",
			desired:     DesiredDir{Path: "/tmp/newdir", Mode: 0o755},
			actual:      nil,
			wantActions: 1,
			wantType:    CreateDir,
		},
		{
			name:    "no change",
			desired: DesiredDir{Path: "/tmp/dir", Mode: 0o755},
			actual: &fact.DirInfo{
				Exists: true,
				Mode:   0o755,
			},
			wantActions: 0,
		},
		{
			name:    "mode changed",
			desired: DesiredDir{Path: "/tmp/dir", Mode: 0o700},
			actual: &fact.DirInfo{
				Exists: true,
				Mode:   0o755,
			},
			wantActions: 1,
			wantType:    SetPermissions,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actions := DiffDir(tt.desired, tt.actual)
			if len(actions) != tt.wantActions {
				t.Fatalf("expected %d actions, got %d: %v", tt.wantActions, len(actions), actions)
			}
			if tt.wantActions > 0 && actions[0].Type != tt.wantType {
				t.Fatalf("expected type %s, got %s", tt.wantType, actions[0].Type)
			}
		})
	}
}
