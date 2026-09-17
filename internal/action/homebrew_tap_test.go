package action

import (
	"slices"
	"testing"

	"github.com/ryanwersal/crucible/internal/fact"
)

func TestValidateHomebrewTaps(t *testing.T) {
	t.Parallel()
	installed := fact.HomebrewPackage{Name: "app", Kind: "cask", Tap: "owner/tools", Names: []string{"app", "owner/tools/app", "old-app"}}
	brew := &fact.HomebrewInfo{Available: true, InstalledPackages: []fact.HomebrewPackage{installed}}
	for _, tc := range []struct {
		name      string
		taps      []DesiredHomebrewTap
		packages  []DesiredPackage
		installed bool
		wantErr   bool
	}{
		{name: "missing undeclared tap", packages: []DesiredPackage{{Name: "owner/tools/new"}}, wantErr: true},
		{name: "explicit add", taps: []DesiredHomebrewTap{{Name: "owner/tools"}}, packages: []DesiredPackage{{Name: "owner/tools/new"}}},
		{name: "already tapped", installed: true, packages: []DesiredPackage{{Name: "owner/tools/new"}}},
		{name: "official API", packages: []DesiredPackage{{Name: "homebrew/core/jq"}}},
		{name: "unmanaged installed app blocks untap", installed: true, taps: []DesiredHomebrewTap{{Name: "owner/tools", Absent: true}}, wantErr: true},
		{name: "remove cask then untap", installed: true, taps: []DesiredHomebrewTap{{Name: "owner/tools", Absent: true}}, packages: []DesiredPackage{{Name: "app", Kind: "cask", Absent: true}}},
		{name: "alias removal", installed: true, taps: []DesiredHomebrewTap{{Name: "owner/tools", Absent: true}}, packages: []DesiredPackage{{Name: "old-app", Kind: "cask", Absent: true}}},
		{name: "wrong kind cannot authorize removal", installed: true, taps: []DesiredHomebrewTap{{Name: "owner/tools", Absent: true}}, packages: []DesiredPackage{{Name: "app", Kind: "formula", Absent: true}}, wantErr: true},
		{name: "wrong tap cannot authorize removal", installed: true, taps: []DesiredHomebrewTap{{Name: "owner/tools", Absent: true}}, packages: []DesiredPackage{{Name: "other/tools/app", Absent: true}}, wantErr: true},
		{name: "keep package conflict", installed: true, taps: []DesiredHomebrewTap{{Name: "owner/tools", Absent: true}}, packages: []DesiredPackage{{Name: "app", Kind: "cask"}}, wantErr: true},
		{name: "conflicting taps", taps: []DesiredHomebrewTap{{Name: "owner/tools"}, {Name: "owner/tools", Absent: true}}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actual := &fact.HomebrewTapInfo{Available: true, Taps: map[string]bool{"owner/tools": tc.installed}}
			err := ValidateHomebrewTaps(tc.taps, tc.packages, actual, brew)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v, wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

func TestDiffAndOrderHomebrewTaps(t *testing.T) {
	t.Parallel()
	acts, obs, err := DiffHomebrewTaps([]DesiredHomebrewTap{
		{Name: "new/tools"}, {Name: "old/tools", Absent: true}, {Name: "keep/tools"}, {Name: "gone/tools", Absent: true}, {Name: "new/tools"},
	}, &fact.HomebrewTapInfo{Available: true, Taps: map[string]bool{"old/tools": true, "keep/tools": true}})
	if err != nil || len(acts) != 2 || len(obs) != 2 {
		t.Fatalf("actions=%v observations=%v err=%v", acts, obs, err)
	}
	actions := []Action{acts[1], {Type: WriteFile}, {Type: InstallPackage, SerialGroup: homebrewSerialGroup}, acts[0], {Type: UninstallPackage, SerialGroup: homebrewSerialGroup}}
	OrderHomebrewActions(actions)
	got := make([]Type, len(actions))
	for i, a := range actions {
		got[i] = a.Type
	}
	if !slices.Equal(got, []Type{AddHomebrewTap, WriteFile, InstallPackage, UninstallPackage, RemoveHomebrewTap}) {
		t.Fatal(got)
	}
}

func TestTypedHomebrewDoesNotConfuseOrigins(t *testing.T) {
	t.Parallel()
	brew := &fact.HomebrewInfo{Available: true, InstalledPackages: []fact.HomebrewPackage{{Name: "app", Kind: "cask", Tap: "one/tools", Names: []string{"app", "one/tools/app"}}}}
	for _, pkg := range []DesiredPackage{{Name: "app", Kind: "formula"}, {Name: "two/tools/app", Kind: "cask"}} {
		acts, _, _, err := DiffHomebrew([]DesiredPackage{pkg}, brew)
		if err != nil || len(acts) != 1 || acts[0].Type != InstallPackage || acts[0].PackageKind != pkg.Kind {
			t.Fatalf("actions=%v err=%v", acts, err)
		}
	}
}
