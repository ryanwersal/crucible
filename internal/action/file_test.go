package action

import (
	"testing"

	"github.com/ryanwersal/crucible/internal/fact"
)

func TestDiffFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		desired     DesiredFile
		actual      *fact.FileInfo
		wantActions int
		wantType    Type
		wantErr     bool
	}{
		{
			name:        "new file",
			desired:     DesiredFile{Path: "/tmp/test.txt", Content: []byte("hello"), Mode: 0o644},
			actual:      nil,
			wantActions: 1,
			wantType:    WriteFile,
		},
		{
			name:    "no change",
			desired: DesiredFile{Path: "/tmp/test.txt", Content: []byte("hello"), Mode: 0o644},
			actual: &fact.FileInfo{
				Exists: true,
				Hash:   sha256Hex([]byte("hello")),
				Mode:   0o644,
			},
			wantActions: 0,
		},
		{
			name:    "content changed",
			desired: DesiredFile{Path: "/tmp/test.txt", Content: []byte("new content"), Mode: 0o644},
			actual: &fact.FileInfo{
				Exists: true,
				Hash:   sha256Hex([]byte("old content")),
				Mode:   0o644,
			},
			wantActions: 1,
			wantType:    WriteFile,
		},
		{
			name:    "mode changed",
			desired: DesiredFile{Path: "/tmp/test.txt", Content: []byte("hello"), Mode: 0o755},
			actual: &fact.FileInfo{
				Exists: true,
				Hash:   sha256Hex([]byte("hello")),
				Mode:   0o644,
			},
			wantActions: 1,
			wantType:    SetPermissions,
		},
		{
			name:    "is directory conflict",
			desired: DesiredFile{Path: "/tmp/test.txt", Content: []byte("hello"), Mode: 0o644},
			actual:  &fact.FileInfo{Exists: true, IsDir: true},
			wantErr: true,
		},
		{
			name:        "replace symlink",
			desired:     DesiredFile{Path: "/tmp/test.txt", Content: []byte("hello"), Mode: 0o644},
			actual:      &fact.FileInfo{Exists: true, IsLink: true},
			wantActions: 2,
			wantType:    DeletePath,
		},
		{
			name:        "absent and file exists",
			desired:     DesiredFile{Path: "/tmp/test.txt", Absent: true},
			actual:      &fact.FileInfo{Exists: true},
			wantActions: 1,
			wantType:    DeletePath,
		},
		{
			name:        "absent and file does not exist",
			desired:     DesiredFile{Path: "/tmp/test.txt", Absent: true},
			actual:      nil,
			wantActions: 0,
		},
		{
			name:        "absent but is directory",
			desired:     DesiredFile{Path: "/tmp/test.txt", Absent: true},
			actual:      &fact.FileInfo{Exists: true, IsDir: true},
			wantActions: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actions, err := DiffFile(tt.desired, tt.actual)
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
				t.Fatalf("expected %d actions, got %d: %v", tt.wantActions, len(actions), actions)
			}
			if tt.wantActions > 0 && actions[0].Type != tt.wantType {
				t.Fatalf("expected first action type %s, got %s", tt.wantType, actions[0].Type)
			}
		})
	}
}
