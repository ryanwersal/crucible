package action

import (
	"strings"
	"testing"

	"github.com/ryanwersal/crucible/internal/fact"
)

func TestDiffVSCode(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name               string
		desired            []DesiredVSCodeExtension
		installed, bundled bool
		want               []Type
		wantErr            string
	}{
		{name: "missing", desired: []DesiredVSCodeExtension{{ID: "Publisher.Extension"}}, want: []Type{InstallVSCodeExtension}},
		{name: "installed", desired: []DesiredVSCodeExtension{{ID: "publisher.extension"}}, installed: true},
		{name: "bundled", desired: []DesiredVSCodeExtension{{ID: "publisher.extension"}}, bundled: true},
		{name: "remove", desired: []DesiredVSCodeExtension{{ID: "publisher.extension", Absent: true}}, installed: true, want: []Type{UninstallVSCodeExtension}},
		{name: "already absent", desired: []DesiredVSCodeExtension{{ID: "publisher.extension", Absent: true}}},
		{name: "cannot remove bundled", desired: []DesiredVSCodeExtension{{ID: "publisher.extension", Absent: true}}, bundled: true, wantErr: "bundled"},
		{name: "duplicates", desired: []DesiredVSCodeExtension{{ID: "Publisher.Extension"}, {ID: "publisher.extension"}}, want: []Type{InstallVSCodeExtension}},
		{name: "conflict", desired: []DesiredVSCodeExtension{{ID: "Publisher.Extension"}, {ID: "publisher.extension", Absent: true}}, wantErr: "conflicting"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := DiffVSCode(tt.desired, &fact.VSCodeInfo{Command: "/bin/code", Extensions: map[string]bool{"publisher.extension": tt.installed}, Bundled: map[string]bool{"publisher.extension": tt.bundled}})
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want %s", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("actions = %+v, want %v", got, tt.want)
			}
			for i, a := range got {
				if a.Type != tt.want[i] || a.VSCodeExtension != "publisher.extension" || a.VSCodeCommand != "/bin/code" || a.SerialGroup != "vscode" {
					t.Fatalf("unexpected action: %+v", a)
				}
			}
		})
	}
}
