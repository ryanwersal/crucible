package action

import (
	"strings"
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
			actual:      &fact.SymlinkInfo{Kind: fact.PathMissing},
			wantActions: 1,
			wantType:    CreateSymlink,
		},
		{
			name:        "no change",
			desired:     DesiredSymlink{Path: "/tmp/link", Target: "/tmp/target"},
			actual:      &fact.SymlinkInfo{Kind: fact.PathSymlink, Target: "/tmp/target"},
			wantActions: 0,
		},
		{
			name:        "target changed",
			desired:     DesiredSymlink{Path: "/tmp/link", Target: "/tmp/new"},
			actual:      &fact.SymlinkInfo{Kind: fact.PathSymlink, Target: "/tmp/old"},
			wantActions: 2,
			wantType:    DeletePath,
		},
		{
			name:        "absent and exists",
			desired:     DesiredSymlink{Path: "/tmp/link", Absent: true},
			actual:      &fact.SymlinkInfo{Kind: fact.PathSymlink, Target: "/tmp/target"},
			wantActions: 1,
			wantType:    DeletePath,
		},
		{
			name:        "absent and does not exist",
			desired:     DesiredSymlink{Path: "/tmp/link", Absent: true},
			actual:      &fact.SymlinkInfo{Kind: fact.PathMissing},
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

func TestDiffSymlink_DestructiveOnRegularFile(t *testing.T) {
	t.Parallel()
	actions := DiffSymlink(
		DesiredSymlink{Path: "/home/u/.zshrc", Target: "/home/u/dotfiles/zshrc"},
		&fact.SymlinkInfo{Kind: fact.PathRegularFile},
	)
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(actions))
	}
	del := actions[0]
	if del.Type != DeletePath {
		t.Fatalf("first action = %s, want DeletePath", del.Type)
	}
	if !del.Destructive {
		t.Error("delete should be Destructive when overwriting a regular file")
	}
	if !strings.Contains(del.DestructiveReason, "regular file") {
		t.Errorf("DestructiveReason should mention the kind: %q", del.DestructiveReason)
	}
	if del.Recursive {
		t.Error("Recursive should be false for a regular file")
	}
	if actions[1].Type != CreateSymlink {
		t.Errorf("second action = %s, want CreateSymlink", actions[1].Type)
	}
}

func TestDiffSymlink_DestructiveOnDirectory(t *testing.T) {
	t.Parallel()
	actions := DiffSymlink(
		DesiredSymlink{Path: "/home/u/.config/fish", Target: "/home/u/dotfiles/fish"},
		&fact.SymlinkInfo{Kind: fact.PathDirectory},
	)
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(actions))
	}
	del := actions[0]
	if !del.Destructive {
		t.Error("delete should be Destructive when overwriting a directory")
	}
	if !del.Recursive {
		t.Error("Recursive must be true to remove a non-empty directory")
	}
	if !strings.Contains(del.DestructiveReason, "directory") {
		t.Errorf("DestructiveReason should mention directory: %q", del.DestructiveReason)
	}
}

func TestDiffSymlink_NoChangeWhenTargetMatches(t *testing.T) {
	t.Parallel()
	actions := DiffSymlink(
		DesiredSymlink{Path: "/tmp/link", Target: "/tmp/target"},
		&fact.SymlinkInfo{Kind: fact.PathSymlink, Target: "/tmp/target"},
	)
	if len(actions) != 0 {
		t.Fatalf("expected no actions, got %d", len(actions))
	}
}

func TestDiffSymlink_AbsentDestructiveOnFile(t *testing.T) {
	t.Parallel()
	actions := DiffSymlink(
		DesiredSymlink{Path: "/home/u/.zshrc", Absent: true},
		&fact.SymlinkInfo{Kind: fact.PathRegularFile},
	)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if !actions[0].Destructive {
		t.Error("absent-over-regular-file must be Destructive")
	}
}

func TestDiffSymlink_AbsentNotDestructiveOnSymlink(t *testing.T) {
	t.Parallel()
	actions := DiffSymlink(
		DesiredSymlink{Path: "/tmp/link", Absent: true},
		&fact.SymlinkInfo{Kind: fact.PathSymlink, Target: "/tmp/target"},
	)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Destructive {
		t.Error("removing a symlink we manage should not be flagged Destructive")
	}
}
