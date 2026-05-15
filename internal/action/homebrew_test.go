package action

import (
	"strings"
	"testing"

	"github.com/ryanwersal/crucible/internal/fact"
)

// Re-export the unexported type alias so test cases can construct OutdatedPackage
// values via the public fact package while keeping diff tests in the action package.
type OutdatedPackage = fact.OutdatedPackage

// lookup mirrors lookupOutdated but returns a zero value (empty Name) when
// no entry exists, so tests can branch on .Name == "" without nil checks.
func lookup(m map[string]OutdatedPackage, name string) OutdatedPackage {
	if p, ok := m[name]; ok {
		return p
	}
	return m[shortName(name)]
}

func TestDiffHomebrew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		desired     []DesiredPackage
		actual      *fact.HomebrewInfo
		wantActions int
		wantErr     bool
	}{
		{
			name:    "brew not available",
			desired: []DesiredPackage{{Name: "git"}},
			actual:  &fact.HomebrewInfo{Available: false},
			wantErr: true,
		},
		{
			name:    "formula already installed",
			desired: []DesiredPackage{{Name: "git"}},
			actual: &fact.HomebrewInfo{
				Available: true,
				Formulae:  map[string]bool{"git": true},
				Casks:     map[string]bool{},
			},
			wantActions: 0,
		},
		{
			name:    "cask already installed",
			desired: []DesiredPackage{{Name: "alacritty"}},
			actual: &fact.HomebrewInfo{
				Available: true,
				Formulae:  map[string]bool{},
				Casks:     map[string]bool{"alacritty": true},
			},
			wantActions: 0,
		},
		{
			name: "needs install",
			desired: []DesiredPackage{
				{Name: "git"},
				{Name: "alacritty"},
			},
			actual: &fact.HomebrewInfo{
				Available: true,
				Formulae:  map[string]bool{"git": true},
				Casks:     map[string]bool{},
			},
			wantActions: 1,
		},
		{
			name:    "tap-qualified name matches short name",
			desired: []DesiredPackage{{Name: "ryanwersal/tools/helios"}},
			actual: &fact.HomebrewInfo{
				Available: true,
				Formulae:  map[string]bool{"helios": true},
				Casks:     map[string]bool{},
			},
			wantActions: 0,
		},
		{
			name:    "tap-qualified name not installed",
			desired: []DesiredPackage{{Name: "ryanwersal/tools/helios"}},
			actual: &fact.HomebrewInfo{
				Available: true,
				Formulae:  map[string]bool{},
				Casks:     map[string]bool{},
			},
			wantActions: 1,
		},
		{
			name:    "absent and installed",
			desired: []DesiredPackage{{Name: "wget", Absent: true}},
			actual: &fact.HomebrewInfo{
				Available: true,
				Formulae:  map[string]bool{"wget": true},
				Casks:     map[string]bool{},
			},
			wantActions: 1,
		},
		{
			name:    "absent and not installed",
			desired: []DesiredPackage{{Name: "wget", Absent: true}},
			actual: &fact.HomebrewInfo{
				Available: true,
				Formulae:  map[string]bool{},
				Casks:     map[string]bool{},
			},
			wantActions: 0,
		},
		{
			name:    "alias matches canonical formula",
			desired: []DesiredPackage{{Name: "kubectl"}},
			actual: &fact.HomebrewInfo{
				Available: true,
				Formulae:  map[string]bool{"kubernetes-cli": true, "kubectl": true},
				Casks:     map[string]bool{},
			},
			wantActions: 0,
		},
		{
			name:    "absent alias uninstalls when alias is in set",
			desired: []DesiredPackage{{Name: "kubectl", Absent: true}},
			actual: &fact.HomebrewInfo{
				Available: true,
				Formulae:  map[string]bool{"kubernetes-cli": true, "kubectl": true},
				Casks:     map[string]bool{},
			},
			wantActions: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actions, _, _, err := DiffHomebrew(tt.desired, tt.actual)
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
		})
	}
}

func TestDiffHomebrew_Latest(t *testing.T) {
	t.Parallel()

	baseFact := func(mod func(*fact.HomebrewInfo)) *fact.HomebrewInfo {
		f := &fact.HomebrewInfo{
			Available: true,
			Formulae:  map[string]bool{"ripgrep": true, "mise": true, "rtx": true, "kubernetes-cli": true, "kubectl": true},
			Casks:     map[string]bool{"orbstack": true, "chrome": true},
			Outdated:  map[string]OutdatedPackage{},
		}
		if mod != nil {
			mod(f)
		}
		return f
	}

	tests := []struct {
		name         string
		desired      []DesiredPackage
		actual       *fact.HomebrewInfo
		wantActions  []Type   // ordered list of expected action types
		wantPackages []string // ordered list of expected PackageName on the actions
		wantObsParts []string // substrings that observation descriptions must each contain
	}{
		{
			name:        "latest but not installed -> install",
			desired:     []DesiredPackage{{Name: "missing", Latest: true}},
			actual:      baseFact(nil),
			wantActions: []Type{InstallPackage},
		},
		{
			name:    "latest, installed, up to date -> no action",
			desired: []DesiredPackage{{Name: "ripgrep", Latest: true}},
			actual:  baseFact(nil),
		},
		{
			name:    "latest, installed, outdated -> upgrade",
			desired: []DesiredPackage{{Name: "ripgrep", Latest: true}},
			actual: baseFact(func(f *fact.HomebrewInfo) {
				f.Outdated["ripgrep"] = OutdatedPackage{Name: "ripgrep", InstalledVersion: "14.1.0", CurrentVersion: "14.1.1"}
			}),
			wantActions:  []Type{UpgradePackage},
			wantPackages: []string{"ripgrep"},
		},
		{
			name:    "latest with alias resolves via outdated map",
			desired: []DesiredPackage{{Name: "kubectl", Latest: true}},
			actual: baseFact(func(f *fact.HomebrewInfo) {
				out := OutdatedPackage{Name: "kubernetes-cli", InstalledVersion: "1.36.0", CurrentVersion: "1.36.1"}
				f.Outdated["kubectl"] = out
				f.Outdated["kubernetes-cli"] = out
			}),
			wantActions:  []Type{UpgradePackage},
			wantPackages: []string{"kubectl"},
		},
		{
			name:    "latest with tap-qualified name falls back to short name",
			desired: []DesiredPackage{{Name: "ryanwersal/tools/helios", Latest: true}},
			actual: baseFact(func(f *fact.HomebrewInfo) {
				f.Formulae["helios"] = true
				f.Formulae["ryanwersal/tools/helios"] = true
				f.Outdated["helios"] = OutdatedPackage{Name: "helios", InstalledVersion: "1.0.0", CurrentVersion: "1.1.0"}
			}),
			wantActions:  []Type{UpgradePackage},
			wantPackages: []string{"ryanwersal/tools/helios"},
		},
		{
			name:    "pinned formula -> observation, no action",
			desired: []DesiredPackage{{Name: "mise", Latest: true}},
			actual: baseFact(func(f *fact.HomebrewInfo) {
				f.Outdated["mise"] = OutdatedPackage{Name: "mise", InstalledVersion: "2026.5.4", CurrentVersion: "2026.5.9", Pinned: true}
			}),
			wantObsParts: []string{"pinned"},
		},
		{
			name:    "auto-updating cask -> observation, no action",
			desired: []DesiredPackage{{Name: "chrome", Latest: true}},
			actual: baseFact(func(f *fact.HomebrewInfo) {
				f.Outdated["chrome"] = OutdatedPackage{Name: "chrome", InstalledVersion: "1.0", CurrentVersion: "2.0", IsCask: true, AutoUpdates: true}
			}),
			wantObsParts: []string{"auto-updates"},
		},
		{
			name: "mixed batch: install, upgrade, skip-pinned, no-op",
			desired: []DesiredPackage{
				{Name: "missing", Latest: true},
				{Name: "ripgrep", Latest: true},
				{Name: "mise", Latest: true},
				{Name: "orbstack", Latest: true},
			},
			actual: baseFact(func(f *fact.HomebrewInfo) {
				f.Outdated["ripgrep"] = OutdatedPackage{Name: "ripgrep", InstalledVersion: "14.1.0", CurrentVersion: "14.1.1"}
				f.Outdated["mise"] = OutdatedPackage{Name: "mise", InstalledVersion: "1", CurrentVersion: "2", Pinned: true}
				// orbstack: present, not outdated → no action, no obs
			}),
			wantActions:  []Type{InstallPackage, UpgradePackage},
			wantPackages: []string{"missing", "ripgrep"},
			wantObsParts: []string{"pinned"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actions, obs, noted, err := DiffHomebrew(tt.desired, tt.actual)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(actions) != len(tt.wantActions) {
				t.Fatalf("actions: got %d (%+v), want %d", len(actions), actions, len(tt.wantActions))
			}
			for i, a := range actions {
				if a.Type != tt.wantActions[i] {
					t.Errorf("action[%d].Type = %v, want %v", i, a.Type, tt.wantActions[i])
				}
				if i < len(tt.wantPackages) && a.PackageName != tt.wantPackages[i] {
					t.Errorf("action[%d].PackageName = %q, want %q", i, a.PackageName, tt.wantPackages[i])
				}
				if a.Type == UpgradePackage {
					if a.PackageInstalledVersion == "" || a.PackageCurrentVersion == "" {
						t.Errorf("UpgradePackage missing version fields: %+v", a)
					}
					if !strings.Contains(a.Description, "→") {
						t.Errorf("UpgradePackage description should show version transition, got %q", a.Description)
					}
				}
			}
			if len(obs) != len(tt.wantObsParts) {
				t.Fatalf("obs: got %d (%+v), want %d", len(obs), obs, len(tt.wantObsParts))
			}
			for i, part := range tt.wantObsParts {
				if !strings.Contains(obs[i].Description, part) {
					t.Errorf("obs[%d] = %q, want substring %q", i, obs[i].Description, part)
				}
			}

			// Every emitted action or observation should be reflected in the
			// noted set (and only those) — that's how the resource layer
			// dedupes "(installed)" notes without parsing descriptions.
			for _, a := range actions {
				if !noted[a.PackageName] {
					t.Errorf("action for %q missing from noted set", a.PackageName)
				}
			}
			for _, pkg := range tt.desired {
				up := lookup(tt.actual.Outdated, pkg.Name)
				skipped := pkg.Latest && (up.Pinned || up.AutoUpdates)
				installed := isInstalled(pkg.Name, tt.actual)
				expectNoted := !pkg.Absent && (!installed || skipped || (pkg.Latest && up.Name != "" && !up.Pinned && !up.AutoUpdates))
				if pkg.Absent && installed {
					expectNoted = true
				}
				if got := noted[pkg.Name]; got != expectNoted {
					t.Errorf("noted[%q] = %v, want %v", pkg.Name, got, expectNoted)
				}
			}
		})
	}
}

func TestShortName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"git", "git"},
		{"ryanwersal/tools/helios", "helios"},
		{"owner/tap/formula", "formula"},
	}

	for _, tt := range tests {
		if got := shortName(tt.input); got != tt.want {
			t.Errorf("shortName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
