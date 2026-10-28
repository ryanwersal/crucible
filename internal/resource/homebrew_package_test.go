package resource

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ryanwersal/crucible/internal/action"
)

func TestHomebrewPackageGuard(t *testing.T) {
	ruby, err := exec.LookPath("ruby")
	if err != nil {
		t.Skip("Ruby is required to exercise Homebrew's adapter")
	}
	dir := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write("tap.rb", `class Tap
 attr_reader :name
 def initialize(name); @name=name; end
 def self.fetch(name); new(name); end
 def install(*); File.write(ENV.fetch("MUTATED_TAP"),name); end
end
`)
	// Homebrew's bundled Ruby does not provide the optional base64 gem.
	write("base64.rb", `raise LoadError, "cannot load such file -- base64"`)
	write("abstract_command.rb", `require "json"
module Homebrew
 class << self; attr_accessor :running_command; end
 def self.failed?; false; end
 module EnvConfig
  def self.no_install_from_api?; true; end
 end
 class AbstractCommand
  def self.command(name); TestCommand; end
 end
 class TestCommand
  def initialize(arguments); @arguments=arguments; end
  def run
   File.write(ENV.fetch("PACKAGE_ARGS"),JSON.generate(@arguments))
   tap=ENV["IMPLICIT_TAP"]
   Tap.fetch(tap).install unless tap.nil? || tap.empty?
  end
 end
end
`)
	for _, op := range []string{"install", "upgrade", "uninstall"} {
		write("cmd/"+op+".rb", "# command loaded\n")
	}
	write("brew", `#!/bin/sh
case "$1" in
 tap) printf 'owner/tools\n';;
 ruby) shift; exec "$TEST_RUBY" -I "$TEST_FIXTURE" "$@";;
 *) exit 98;;
esac
`)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TEST_RUBY", ruby)
	t.Setenv("TEST_FIXTURE", dir)
	t.Setenv("MUTATED_TAP", filepath.Join(dir, "tap-added"))
	t.Setenv("PACKAGE_ARGS", filepath.Join(dir, "args"))
	for _, tc := range []struct {
		name, op, kind, packageName, implicit string
		wantErr                               bool
	}{
		{name: "formula", op: "install", kind: "formula", packageName: "owner/tools/app"},
		{name: "cask", op: "install", kind: "cask", packageName: "owner/tools/app"},
		{name: "upgrade", op: "upgrade", kind: "cask", packageName: "owner/tools/app"},
		{name: "uninstall", op: "uninstall", packageName: "app"},
		{name: "dependency tap blocked", op: "install", kind: "formula", packageName: "app", implicit: "dependency/tools", wantErr: true},
		{name: "cask dependency tap blocked", op: "install", kind: "cask", packageName: "app", implicit: "dependency/tools", wantErr: true},
		{name: "official checkout blocked", op: "upgrade", packageName: "app", implicit: "homebrew/core", wantErr: true},
		{name: "uninstall cannot retap", op: "uninstall", packageName: "app", implicit: "missing/tools", wantErr: true},
		{name: "missing direct tap", op: "install", packageName: "missing/tools/app", wantErr: true},
		{name: "data is not Ruby", op: "install", packageName: `app\"; raise "injection"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("IMPLICIT_TAP", tc.implicit)
			a := action.Action{PackageName: tc.packageName, PackageKind: tc.kind}
			err := runHomebrewPackage(t.Context(), a, nil, io.Discard, io.Discard, tc.op)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if _, err := os.Stat(filepath.Join(dir, "tap-added")); !os.IsNotExist(err) {
				t.Fatalf("tap guard did not prevent mutation: %v", err)
			}
			if !tc.wantErr {
				args, err := os.ReadFile(filepath.Join(dir, "args"))
				if err != nil {
					t.Fatal(err)
				}
				var got []string
				if err := json.Unmarshal(args, &got); err != nil {
					t.Fatal(err)
				}
				want := []string{"--", tc.packageName}
				if tc.kind != "" {
					want = append([]string{"--" + tc.kind}, want...)
				}
				if !slices.Equal(got, want) {
					t.Fatalf("arguments=%q want %q", got, want)
				}
			}
		})
	}
}

func TestRemoveHomebrewTapRechecksRemainingPackages(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "untapped")
	t.Setenv("UNTAP_LOG", log)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	brew := `#!/bin/sh
case "$1" in
 info) printf '%s\n' "$INSTALLED_PACKAGES";;
 outdated) printf '{"formulae":[],"casks":[]}\n';;
 untap) printf '%s\n' "$2" > "$UNTAP_LOG";;
 *) exit 99;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "brew"), []byte(brew), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("INSTALLED_PACKAGES", `{"formulae":[],"casks":[{"token":"app","tap":"owner/tools"}]}`)
	a := action.Action{TapName: "owner/tools"}
	if err := (RemoveHomebrewTapExecutor{}).Execute(t.Context(), a, nil, io.Discard, io.Discard); err == nil {
		t.Fatal("untapped repository with installed cask")
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Fatal("untap ran")
	}
	t.Setenv("INSTALLED_PACKAGES", `{"formulae":[],"casks":[]}`)
	if err := (RemoveHomebrewTapExecutor{}).Execute(t.Context(), a, nil, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(log)
	if err != nil || string(data) != "owner/tools\n" {
		t.Fatalf("data=%s err=%v", data, err)
	}
}

func TestHomebrewPackageEnv(t *testing.T) {
	keys := []string{"HOMEBREW_NO_AUTO_UPDATE", "HOMEBREW_NO_INSTALLED_DEPENDENTS_CHECK", "HOMEBREW_NO_INSTALL_CLEANUP", "HOMEBREW_NO_ENV_HINTS"}
	for _, key := range keys {
		t.Setenv(key, "0")
	}
	t.Setenv("CRUCIBLE_ENV_TEST", "preserved")
	env := homebrewPackageEnv()
	for _, key := range keys {
		count := 0
		for _, item := range env {
			if strings.HasPrefix(item, key+"=") {
				count++
				if item != key+"=1" {
					t.Errorf("unexpected override: %s", item)
				}
			}
		}
		if count != 1 {
			t.Errorf("%s occurs %d times", key, count)
		}
	}
	if !slices.Contains(env, "CRUCIBLE_ENV_TEST=preserved") {
		t.Fatal("lost unrelated environment")
	}
}
