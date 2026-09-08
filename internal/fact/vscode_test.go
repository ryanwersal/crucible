package fact

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVSCodeCollector(t *testing.T) {
	for _, tt := range []struct {
		name, output                    string
		missing, fail, malformed, linux bool
	}{
		{name: "installed and bundled", output: "Publisher.Extension\r\n\n"},
		{name: "Linux installed and bundled", output: "Publisher.Extension\r\n", linux: true},
		{name: "missing CLI", missing: true},
		{name: "CLI failure", fail: true},
		{name: "invalid manifest", malformed: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			bin := filepath.Join(root, "bin")
			if err := os.MkdirAll(bin, 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin)
			if !tt.missing {
				app := filepath.Join(root, "app")
				if err := os.MkdirAll(filepath.Join(app, "bin"), 0o755); err != nil {
					t.Fatal(err)
				}
				code := filepath.Join(app, "bin", "code")
				body := "#!/bin/sh\n[ \"$1\" = --list-extensions ] || exit 2\nprintf '%s' '" + tt.output + "'\n"
				if tt.fail {
					body += "exit 1\n"
				}
				if err := os.WriteFile(code, []byte(body), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(code, filepath.Join(bin, "code")); err != nil {
					t.Fatal(err)
				}
				dir := filepath.Join(app, "extensions", "copilot")
				if tt.linux {
					dir = filepath.Join(app, "resources", "app", "extensions", "copilot")
				}
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				manifest := `{"publisher":"GitHub","name":"copilot-chat"}`
				if tt.malformed {
					manifest = "{"
				}
				if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(manifest), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			info, err := (VSCodeCollector{}).Collect(t.Context())
			if tt.fail || tt.malformed {
				if err == nil {
					t.Fatal("expected collection error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tt.missing {
				if info.Command != "" {
					t.Fatal("CLI should be unavailable")
				}
				return
			}
			if !info.Extensions["publisher.extension"] || !info.Bundled["github.copilot-chat"] {
				t.Fatalf("unexpected facts: %+v", info)
			}
			if !strings.HasSuffix(info.Command, "/bin/code") {
				t.Fatal(info.Command)
			}
		})
	}
}
