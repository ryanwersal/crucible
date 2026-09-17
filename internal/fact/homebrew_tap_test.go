package fact

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHomebrewTapCollector(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	brew := filepath.Join(dir, "brew")
	for _, tc := range []struct {
		name, output string
		fail         bool
	}{
		{name: "empty"}, {name: "installed", output: "owner/tools\nother/repo\n"}, {name: "failure", fail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			script := "#!/bin/sh\n[ \"$*\" = tap ] || exit 9\n"
			if tc.fail {
				script += "echo failure >&2\nexit 1\n"
			} else {
				script += "printf '" + tc.output + "'\n"
			}
			if err := os.WriteFile(brew, []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			info, err := (HomebrewTapCollector{}).Collect(t.Context())
			if tc.fail {
				if err == nil {
					t.Fatal("expected collection error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !info.Available || (tc.output != "" && !info.Taps["owner/tools"]) {
				t.Fatal(info)
			}
		})
	}
}

func TestHomebrewInfoPreservesPackageOrigins(t *testing.T) {
	t.Parallel()
	info, _, err := parseHomebrewInfo([]byte(`{"formulae":[{"name":"tool","full_name":"one/tools/tool","tap":"one/tools","aliases":["alias"]}],"casks":[{"token":"tool","tap":"two/tools"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(info.InstalledPackages) != 2 || info.InstalledPackages[0].Kind != "formula" || info.InstalledPackages[1].Kind != "cask" || info.InstalledPackages[1].Tap != "two/tools" {
		t.Fatal(info)
	}
	if !info.Formulae["one/tools/tool"] || !info.Casks["two/tools/tool"] {
		t.Fatal(info)
	}
}
