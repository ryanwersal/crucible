package resource

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/script/decl"
)

func TestVSCodeExtensionHandler(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, command              string
		installed, bundled, absent bool
		actions                    int
		observation                string
	}{
		{name: "missing CLI", observation: "code not available"},
		{name: "install", command: "code", actions: 1},
		{name: "installed", command: "code", installed: true, observation: "installed"},
		{name: "bundled", command: "code", bundled: true, observation: "bundled"},
		{name: "remove", command: "code", installed: true, absent: true, actions: 1},
		{name: "already absent", command: "code", absent: true, observation: "already absent"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store := fact.NewStore()
			fact.Set(store, "vscode", &fact.VSCodeInfo{Command: tt.command, Extensions: map[string]bool{"publisher.extension": tt.installed}, Bundled: map[string]bool{"publisher.extension": tt.bundled}})
			state := decl.Present
			if tt.absent {
				state = decl.Absent
			}
			out, err := DefaultRegistry().PlanBatch(t.Context(), store, Env{}, decl.VSCodeExtension, []decl.Declaration{{Type: decl.VSCodeExtension, VSCodeExtension: "publisher.extension", State: state}})
			if err != nil {
				t.Fatal(err)
			}
			if len(out.Actions) != tt.actions {
				t.Fatalf("actions = %+v", out.Actions)
			}
			if tt.observation != "" && (len(out.Observations) != 1 || !strings.Contains(out.Observations[0].Description, tt.observation)) {
				t.Fatalf("observations = %+v", out.Observations)
			}
		})
	}
}

func TestVSCodeExecutors(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		kind action.Type
		flag string
		fail bool
	}{
		{name: "install", kind: action.InstallVSCodeExtension, flag: "--install-extension"},
		{name: "uninstall", kind: action.UninstallVSCodeExtension, flag: "--uninstall-extension"},
		{name: "failure", kind: action.InstallVSCodeExtension, flag: "--install-extension", fail: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			code := filepath.Join(t.TempDir(), "code")
			body := "#!/bin/sh\nprintf '%s\\n' \"$@\"\n"
			if tt.fail {
				body += "exit 1\n"
			}
			if err := os.WriteFile(code, []byte(body), 0o755); err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			err := DefaultRegistry().Execute(t.Context(), action.Action{Type: tt.kind, VSCodeCommand: code, VSCodeExtension: "publisher.extension"}, nil, &out, io.Discard)
			if (err != nil) != tt.fail {
				t.Fatalf("error = %v", err)
			}
			if out.String() != tt.flag+"\npublisher.extension\n" {
				t.Fatalf("arguments = %q", out.String())
			}
		})
	}
}
