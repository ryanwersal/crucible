package fact

import (
	"testing"
)

func TestParseHomebrewOutdated(t *testing.T) {
	t.Parallel()

	// Install info establishes alias groups (so outdated entries replay
	// across every name a user might declare) plus cask auto_updates flags
	// (which brew outdated doesn't emit).
	installInput := `{"formulae":[
		{"name":"kubernetes-cli","full_name":"kubernetes-cli","aliases":["kubectl"],"oldnames":[]},
		{"name":"mise","full_name":"mise","aliases":[],"oldnames":["rtx"]},
		{"name":"ripgrep","full_name":"ripgrep","aliases":[],"oldnames":[]},
		{"name":"helios","full_name":"ryanwersal/tools/helios","aliases":[],"oldnames":[]}
	],"casks":[
		{"token":"chrome","full_token":"chrome","old_tokens":[],"auto_updates":true},
		{"token":"orbstack","full_token":"orbstack","old_tokens":[],"auto_updates":false}
	]}`

	_, ctx, err := parseHomebrewInfo([]byte(installInput))
	if err != nil {
		t.Fatalf("seed parseHomebrewInfo: %v", err)
	}

	tests := []struct {
		name    string
		input   string
		check   func(t *testing.T, out map[string]OutdatedPackage)
		wantErr bool
	}{
		{
			name:  "empty",
			input: `{"formulae":[],"casks":[]}`,
			check: func(t *testing.T, out map[string]OutdatedPackage) {
				if len(out) != 0 {
					t.Errorf("expected empty map, got %v", out)
				}
			},
		},
		{
			name: "formula outdated replays onto alias",
			input: `{"formulae":[{
				"name":"kubernetes-cli","installed_versions":["1.36.0"],
				"current_version":"1.36.1","pinned":false
			}],"casks":[]}`,
			check: func(t *testing.T, out map[string]OutdatedPackage) {
				want := OutdatedPackage{
					Name: "kubernetes-cli", InstalledVersion: "1.36.0",
					CurrentVersion: "1.36.1",
				}
				for _, key := range []string{"kubernetes-cli", "kubectl"} {
					got, ok := out[key]
					if !ok {
						t.Fatalf("missing entry for %q", key)
					}
					if got != want {
						t.Errorf("%q: got %+v, want %+v", key, got, want)
					}
				}
			},
		},
		{
			name: "pinned formula flagged",
			input: `{"formulae":[{
				"name":"mise","installed_versions":["2026.5.4"],
				"current_version":"2026.5.9","pinned":true
			}],"casks":[]}`,
			check: func(t *testing.T, out map[string]OutdatedPackage) {
				got, ok := out["mise"]
				if !ok {
					t.Fatal("missing mise entry")
				}
				if !got.Pinned {
					t.Errorf("want pinned=true, got %+v", got)
				}
				if !out["rtx"].Pinned {
					t.Errorf("pinned flag should replay onto oldname rtx, got %+v", out["rtx"])
				}
			},
		},
		{
			name: "tap-qualified formula keyed by both canonical and full name",
			input: `{"formulae":[{
				"name":"helios","installed_versions":["1.0.0"],
				"current_version":"1.1.0","pinned":false
			}],"casks":[]}`,
			check: func(t *testing.T, out map[string]OutdatedPackage) {
				for _, key := range []string{"helios", "ryanwersal/tools/helios"} {
					if _, ok := out[key]; !ok {
						t.Errorf("missing entry for %q (got keys %v)", key, keysOf(out))
					}
				}
			},
		},
		{
			name: "cask auto_updates picked up from install context",
			input: `{"formulae":[],"casks":[
				{"name":"chrome","installed_versions":["1.0"],"current_version":"2.0"}
			]}`,
			check: func(t *testing.T, out map[string]OutdatedPackage) {
				got, ok := out["chrome"]
				if !ok {
					t.Fatal("missing chrome entry")
				}
				if !got.IsCask {
					t.Error("want IsCask=true")
				}
				if !got.AutoUpdates {
					t.Error("want AutoUpdates=true (chrome is flagged in install info)")
				}
			},
		},
		{
			name: "cask without auto_updates flag stays false",
			input: `{"formulae":[],"casks":[
				{"name":"orbstack","installed_versions":["1.7.2"],"current_version":"1.8.0"}
			]}`,
			check: func(t *testing.T, out map[string]OutdatedPackage) {
				got := out["orbstack"]
				if got.AutoUpdates {
					t.Errorf("want AutoUpdates=false, got %+v", got)
				}
				if got.InstalledVersion != "1.7.2" || got.CurrentVersion != "1.8.0" {
					t.Errorf("version mismatch: %+v", got)
				}
			},
		},
		{
			name: "unknown package falls back to canonical-only key",
			input: `{"formulae":[{
				"name":"never-installed","installed_versions":["0.1"],
				"current_version":"0.2","pinned":false
			}],"casks":[]}`,
			check: func(t *testing.T, out map[string]OutdatedPackage) {
				if _, ok := out["never-installed"]; !ok {
					t.Error("entry should still be recorded under canonical name")
				}
			},
		},
		{
			name:    "malformed json",
			input:   `{"formulae":`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out, err := parseHomebrewOutdated([]byte(tt.input), ctx)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.check(t, out)
		})
	}
}

func keysOf(m map[string]OutdatedPackage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestParseHomebrewInfo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		input        string
		wantFormulae []string // names that must all resolve as installed
		wantCasks    []string
		wantMissing  []string // names that must NOT resolve (negative control)
		wantErr      bool
	}{
		{
			name:  "empty",
			input: `{"formulae":[],"casks":[]}`,
		},
		{
			name: "alias resolves to canonical formula",
			input: `{"formulae":[{
				"name":"kubernetes-cli",
				"full_name":"kubernetes-cli",
				"aliases":["kubectl","kubernetes-cli@1.35"],
				"oldnames":[]
			}],"casks":[]}`,
			wantFormulae: []string{"kubernetes-cli", "kubectl", "kubernetes-cli@1.35"},
			wantMissing:  []string{"not-installed"},
		},
		{
			name: "oldname resolves to current formula",
			input: `{"formulae":[{
				"name":"mise",
				"full_name":"mise",
				"aliases":[],
				"oldnames":["rtx"]
			}],"casks":[]}`,
			wantFormulae: []string{"mise", "rtx"},
		},
		{
			name: "tap-qualified full_name and short name both match",
			input: `{"formulae":[{
				"name":"helios",
				"full_name":"ryanwersal/tools/helios",
				"aliases":[],
				"oldnames":[]
			}],"casks":[]}`,
			wantFormulae: []string{"helios", "ryanwersal/tools/helios"},
		},
		{
			name: "cask with old_tokens",
			input: `{"formulae":[],"casks":[{
				"token":"todoist-app",
				"full_token":"todoist-app",
				"old_tokens":["todoist"]
			}]}`,
			wantCasks: []string{"todoist-app", "todoist"},
		},
		{
			name: "multiple formulae and casks",
			input: `{"formulae":[
				{"name":"git-delta","full_name":"git-delta","aliases":["delta"],"oldnames":[]},
				{"name":"ripgrep","full_name":"ripgrep","aliases":["rg"],"oldnames":[]}
			],"casks":[
				{"token":"alacritty","full_token":"alacritty","old_tokens":[]}
			]}`,
			wantFormulae: []string{"git-delta", "delta", "ripgrep", "rg"},
			wantCasks:    []string{"alacritty"},
			wantMissing:  []string{"alacritty-dev"},
		},
		{
			name:    "malformed json",
			input:   `{"formulae":`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			info, _, err := parseHomebrewInfo([]byte(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for _, name := range tt.wantFormulae {
				if !info.Formulae[name] {
					t.Errorf("formula %q: want present, missing from %v", name, info.Formulae)
				}
			}
			for _, name := range tt.wantCasks {
				if !info.Casks[name] {
					t.Errorf("cask %q: want present, missing from %v", name, info.Casks)
				}
			}
			for _, name := range tt.wantMissing {
				if info.Formulae[name] || info.Casks[name] {
					t.Errorf("%q: want absent, found in sets", name)
				}
			}
		})
	}
}
