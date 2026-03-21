package action

import (
	"testing"

	"github.com/ryanwersal/crucible/internal/fact"
)

func TestDiffKeyRemap(t *testing.T) {
	t.Parallel()

	capsLockCode := uint64(0x700000039)
	controlCode := uint64(0x7000000E0)

	tests := []struct {
		name       string
		desired    DesiredKeyRemap
		actual     *fact.KeyRemapInfo
		wantCount  int
		wantType   Type
	}{
		{
			name: "no remaps needed, nil actual",
			desired: DesiredKeyRemap{
				Remaps: []KeyRemapEntry{{From: "capsLock", To: "control"}},
			},
			actual:    nil,
			wantCount: 1,
			wantType:  SetKeyRemap,
		},
		{
			name: "already up to date",
			desired: DesiredKeyRemap{
				Remaps: []KeyRemapEntry{{From: "capsLock", To: "control"}},
			},
			actual: &fact.KeyRemapInfo{
				Mappings: []fact.KeyRemapMapping{
					{Src: capsLockCode, Dst: controlCode},
				},
			},
			wantCount: 0,
		},
		{
			name: "different mapping active",
			desired: DesiredKeyRemap{
				Remaps: []KeyRemapEntry{{From: "capsLock", To: "control"}},
			},
			actual: &fact.KeyRemapInfo{
				Mappings: []fact.KeyRemapMapping{
					{Src: controlCode, Dst: capsLockCode},
				},
			},
			wantCount: 1,
			wantType:  SetKeyRemap,
		},
		{
			name: "absent with existing mappings",
			desired: DesiredKeyRemap{Absent: true},
			actual: &fact.KeyRemapInfo{
				Mappings: []fact.KeyRemapMapping{
					{Src: capsLockCode, Dst: controlCode},
				},
			},
			wantCount: 1,
			wantType:  RemoveKeyRemap,
		},
		{
			name:      "absent with no existing mappings",
			desired:   DesiredKeyRemap{Absent: true},
			actual:    &fact.KeyRemapInfo{},
			wantCount: 0,
		},
		{
			name:      "absent with nil actual",
			desired:   DesiredKeyRemap{Absent: true},
			actual:    nil,
			wantCount: 0,
		},
		{
			name: "empty actual mappings",
			desired: DesiredKeyRemap{
				Remaps: []KeyRemapEntry{{From: "capsLock", To: "control"}},
			},
			actual:    &fact.KeyRemapInfo{Mappings: []fact.KeyRemapMapping{}},
			wantCount: 1,
			wantType:  SetKeyRemap,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actions := DiffKeyRemap(tt.desired, tt.actual, "/tmp/test.plist")
			if len(actions) != tt.wantCount {
				t.Fatalf("expected %d actions, got %d: %v", tt.wantCount, len(actions), actions)
			}
			if tt.wantCount > 0 && actions[0].Type != tt.wantType {
				t.Errorf("expected %s, got %s", tt.wantType, actions[0].Type)
			}
		})
	}
}
