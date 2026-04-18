package fact

import (
	"testing"
)

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
			info, err := parseHomebrewInfo([]byte(tt.input))
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
