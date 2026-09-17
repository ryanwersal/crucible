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
	code := homebrewPackageGuard + "\nrequire 'base64'; require 'json'; CrucibleHomebrewPackage.run(" +
		fmt.Sprintf("%q", operation) + ", JSON.parse(Base64.strict_decode64(\"" + base64.StdEncoding.EncodeToString(encoded) + "\")))\n"
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
	env := make([]string, 0, len(os.Environ())+2)
	for _, item := range os.Environ() {
		if strings.HasPrefix(item, "HOMEBREW_NO_AUTO_UPDATE=") || strings.HasPrefix(item, "HOMEBREW_NO_INSTALLED_DEPENDENTS_CHECK=") {
			continue
		}
		env = append(env, item)
	}
	return append(env, "HOMEBREW_NO_AUTO_UPDATE=1", "HOMEBREW_NO_INSTALLED_DEPENDENTS_CHECK=1")
}
