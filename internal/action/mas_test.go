package action

import (
	"testing"

	"github.com/ryanwersal/crucible/internal/fact"
)

func TestDiffMas(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		desired     []DesiredMasApp
		actual      *fact.MasInfo
		wantActions int
		wantErr     bool
	}{
		{
			name:    "mas not available",
			desired: []DesiredMasApp{{ID: 497799835, Name: "Xcode"}},
			actual:  &fact.MasInfo{Available: false},
			wantErr: true,
		},
		{
			name:    "app already installed",
			desired: []DesiredMasApp{{ID: 497799835, Name: "Xcode"}},
			actual: &fact.MasInfo{
				Available: true,
				Apps:      map[int64]string{497799835: "Xcode"},
			},
			wantActions: 0,
		},
		{
			name:    "app missing",
			desired: []DesiredMasApp{{ID: 497799835, Name: "Xcode"}},
			actual: &fact.MasInfo{
				Available: true,
				Apps:      map[int64]string{},
			},
			wantActions: 1,
		},
		{
			name: "mixed installed and missing",
			desired: []DesiredMasApp{
				{ID: 497799835, Name: "Xcode"},
				{ID: 409183694, Name: "Keynote"},
			},
			actual: &fact.MasInfo{
				Available: true,
				Apps:      map[int64]string{497799835: "Xcode"},
			},
			wantActions: 1,
		},
		{
			name:    "app missing without name",
			desired: []DesiredMasApp{{ID: 497799835}},
			actual: &fact.MasInfo{
				Available: true,
				Apps:      map[int64]string{},
			},
			wantActions: 1,
		},
		{
			name:        "empty desired list",
			desired:     nil,
			actual:      &fact.MasInfo{Available: true, Apps: map[int64]string{}},
			wantActions: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actions, err := DiffMas(tt.desired, tt.actual)
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
				if a.Type != InstallMasApp {
					t.Errorf("expected InstallMasApp type, got %v", a.Type)
				}
			}
		})
	}
}
