package modules

import (
	"testing"

	"github.com/ryanwersal/crucible/internal/script/decl"
)

func TestVSCode(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, script    string
		count           int
		absent, invalid bool
	}{
		{name: "single", script: `c.vscode("GitHub.copilot-chat")`, count: 1},
		{name: "array", script: `c.vscode(["GitHub.copilot-chat","GitHub.vscode-pull-request-github"])`, count: 2},
		{name: "absent", script: `c.vscode("GitHub.copilot-chat", {state:"absent"})`, count: 1, absent: true},
		{name: "empty list", script: `c.vscode([])`},
		{name: "missing", script: `c.vscode()`, invalid: true},
		{name: "option injection", script: `c.vscode("--force")`, invalid: true},
		{name: "path", script: `c.vscode("./extension.vsix")`, invalid: true},
		{name: "version", script: `c.vscode("GitHub.copilot-chat@1.0.0")`, invalid: true},
		{name: "invalid array", script: `c.vscode(["GitHub.copilot-chat",42])`, invalid: true},
		{name: "latest", script: `c.vscode("GitHub.copilot-chat",{state:"latest"})`, invalid: true},
		{name: "unknown option", script: `c.vscode("GitHub.copilot-chat",{profile:"work"})`, invalid: true},
		{name: "null options", script: `c.vscode("GitHub.copilot-chat",null)`, invalid: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			vm, ds := setupModule(t)
			_, err := vm.RunString(tt.script)
			if (err != nil) != tt.invalid {
				t.Fatalf("error = %v, invalid = %v", err, tt.invalid)
			}
			if len(*ds) != tt.count {
				t.Fatalf("declarations = %v", *ds)
			}
			for _, d := range *ds {
				if d.Type != decl.VSCodeExtension || (d.State == decl.Absent) != tt.absent {
					t.Fatalf("unexpected declaration: %+v", d)
				}
			}
			if tt.count > 0 && (*ds)[0].VSCodeExtension != "github.copilot-chat" {
				t.Fatalf("ID = %s", (*ds)[0].VSCodeExtension)
			}
		})
	}
}
