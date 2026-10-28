package resource

import (
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/fact"
)

//go:embed homebrew_package.rb
var homebrewPackageGuard string

func runHomebrewPackage(ctx context.Context, a action.Action, stdin io.Reader, stdout, stderr io.Writer, operation string) error {
	taps, err := (fact.HomebrewTapCollector{}).Collect(ctx)
	if err != nil {
		return err
	}
	tap := action.PackageTap(a.PackageName)
	if tap != "" && tap != "homebrew/core" && tap != "homebrew/cask" && !taps.Taps[tap] {
		return fmt.Errorf("package %s requires tap %s; add c.brew.tap(%q) before installing packages", a.PackageName, tap, tap)
	}
	arguments := []string{}
	if a.PackageKind != "" {
		arguments = append(arguments, "--"+a.PackageKind)
	}
	arguments = append(arguments, "--", a.PackageName)
	encoded, err := json.Marshal(arguments)
	if err != nil {
		return err
	}
	// Encode data separately from Ruby source: package names cannot inject code.
	// Use Ruby's built-in decoder: Homebrew does not bundle the base64 gem.
	code := homebrewPackageGuard + "\nrequire 'json'; CrucibleHomebrewPackage.run(" +
		fmt.Sprintf("%q", operation) + ", JSON.parse(\"" + base64.StdEncoding.EncodeToString(encoded) + "\".unpack1('m0')))\n"
	// Don't let Homebrew's shell launcher update/migrate taps before our guard
	// loads. Explicit refresh remains a separate planning step.
	err = runCmdEnv(ctx, a, stdin, stdout, stderr, homebrewPackageEnv(), "brew", "ruby", "-e", code)
	if commandErr, ok := errors.AsType[*CommandError](err); ok {
		// Keep user-facing errors about the package operation, not the adapter.
		simplified := *commandErr
		simplified.Args = append([]string{operation}, arguments...)
		return &simplified
	}
	return err
}

func homebrewPackageEnv() []string {
	overrides := []string{"HOMEBREW_NO_AUTO_UPDATE", "HOMEBREW_NO_INSTALLED_DEPENDENTS_CHECK", "HOMEBREW_NO_INSTALL_CLEANUP", "HOMEBREW_NO_ENV_HINTS"}
	env := make([]string, 0, len(os.Environ())+len(overrides))
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if slices.Contains(overrides, key) {
			continue
		}
		env = append(env, item)
	}
	for _, key := range overrides {
		env = append(env, key+"=1")
	}
	return env
}
