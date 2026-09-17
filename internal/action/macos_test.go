package action

import (
	"testing"

	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/script/decl"
)

func TestDiffMacOS(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		desired   decl.MacOSTweaks
		actual    *fact.MacOSInfo
		wantCount int
		wantSudo  bool
	}{
		{
			name:      "nothing declared",
			desired:   decl.MacOSTweaks{},
			actual:    &fact.MacOSInfo{SpotlightIndexing: true, MusicPlayKey: true, ClickWallpaperToShowDesktop: true},
			wantCount: 0,
		},
		{
			name:      "spotlight already disabled",
			desired:   decl.MacOSTweaks{SpotlightIndexing: new(false)},
			actual:    &fact.MacOSInfo{SpotlightIndexing: false},
			wantCount: 0,
		},
		{
			name:      "spotlight needs disabling requires sudo",
			desired:   decl.MacOSTweaks{SpotlightIndexing: new(false)},
			actual:    &fact.MacOSInfo{SpotlightIndexing: true},
			wantCount: 1,
			wantSudo:  true,
		},
		{
			name:      "music play key needs disabling without sudo",
			desired:   decl.MacOSTweaks{MusicPlayKey: new(false)},
			actual:    &fact.MacOSInfo{MusicPlayKey: true},
			wantCount: 1,
		},
		{
			name: "all three differ",
			desired: decl.MacOSTweaks{
				SpotlightIndexing:           new(false),
				MusicPlayKey:                new(false),
				ClickWallpaperToShowDesktop: new(false),
			},
			actual:    &fact.MacOSInfo{SpotlightIndexing: true, MusicPlayKey: true, ClickWallpaperToShowDesktop: true},
			wantCount: 3,
		},
		{
			name:      "nil actual treated as everything off",
			desired:   decl.MacOSTweaks{SpotlightIndexing: new(true)},
			actual:    nil,
			wantCount: 1,
			wantSudo:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			acts := DiffMacOS(tt.desired, tt.actual)
			if len(acts) != tt.wantCount {
				t.Fatalf("got %d actions, want %d", len(acts), tt.wantCount)
			}
			if tt.wantCount == 1 {
				if acts[0].Type != SetMacOSTweak {
					t.Errorf("type = %v, want SetMacOSTweak", acts[0].Type)
				}
				if acts[0].SerialGroup != "macos:"+string(acts[0].MacOSTweak) {
					t.Errorf("SerialGroup = %q", acts[0].SerialGroup)
				}
				if acts[0].NeedsSudo != tt.wantSudo {
					t.Errorf("NeedsSudo = %v, want %v", acts[0].NeedsSudo, tt.wantSudo)
				}
			}
		})
	}
}

func TestDiffMacOSIncompleteState(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name    string
		desired decl.MacOSTweaks
		actual  fact.MacOSInfo
	}{
		{"enable mixed volumes", decl.MacOSTweaks{SpotlightIndexing: new(true)}, fact.MacOSInfo{SpotlightIndexing: true, SpotlightIndexingMixed: true}},
		{"disable mixed volumes", decl.MacOSTweaks{SpotlightIndexing: new(false)}, fact.MacOSInfo{SpotlightIndexing: true, SpotlightIndexingMixed: true}},
		{"retry failed bootstrap", decl.MacOSTweaks{MusicPlayKey: new(true)}, fact.MacOSInfo{MusicPlayKey: true, MusicPlayKeyLoaded: false}},
		{"retry failed bootout", decl.MacOSTweaks{MusicPlayKey: new(false)}, fact.MacOSInfo{MusicPlayKey: false, MusicPlayKeyLoaded: true}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			acts := DiffMacOS(tt.desired, &tt.actual)
			if len(acts) != 1 {
				t.Fatalf("actions = %+v", acts)
			}
		})
	}
}

func TestSpotlightOnlyBackupStoresAreUnmanaged(t *testing.T) {
	t.Parallel()
	for _, enabled := range []bool{false, true} {
		actual := &fact.MacOSInfo{SpotlightExcluded: []string{"/Volumes/Backup"}}
		if actions := DiffMacOS(decl.MacOSTweaks{SpotlightIndexing: &enabled}, actual); len(actions) != 0 {
			t.Fatalf("enabled=%t produced actions for excluded stores: %+v", enabled, actions)
		}
	}
}
