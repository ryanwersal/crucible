package action

import (
	"fmt"
	"strings"

	"github.com/ryanwersal/crucible/internal/fact"
)

// DesiredPackage describes a Homebrew package that should be installed (or removed).
type DesiredPackage struct {
	Name   string // may be a tap-qualified name like "owner/tap/formula"
	Absent bool   // true = ensure the package is not installed
}

// DiffHomebrew compares desired packages against installed state.
func DiffHomebrew(desired []DesiredPackage, actual *fact.HomebrewInfo) ([]Action, error) {
	if !actual.Available {
		return nil, fmt.Errorf("homebrew is required but not installed")
	}

	var actions []Action
	for _, pkg := range desired {
		installed := isInstalled(pkg.Name, actual)
		if pkg.Absent {
			if installed {
				actions = append(actions, Action{
					Type:        UninstallPackage,
					PackageName: pkg.Name,
					Description: fmt.Sprintf("brew uninstall %s", pkg.Name),
				})
			}
			continue
		}
		if !installed {
			actions = append(actions, Action{
				Type:        InstallPackage,
				PackageName: pkg.Name,
				Description: fmt.Sprintf("brew install %s", pkg.Name),
			})
		}
	}
	return actions, nil
}

// isInstalled checks whether a package is present in either formulae or casks.
// The fact's Formulae/Casks sets include canonical names, full_names/tokens,
// aliases, and oldnames — so aliased formulas like "kubectl" (→ kubernetes-cli)
// and tap-qualified names both match directly. shortName() is the fallback for
// the legacy case where a tap-qualified desired name predates alias-aware facts.
func isInstalled(name string, actual *fact.HomebrewInfo) bool {
	if actual.Formulae[name] || actual.Casks[name] {
		return true
	}
	short := shortName(name)
	return actual.Formulae[short] || actual.Casks[short]
}

// shortName extracts the trailing formula/cask name from a tap-qualified
// package name. "owner/tap/formula" → "formula". Plain names are unchanged.
func shortName(name string) string {
	if i := strings.LastIndex(name, "/"); i >= 0 {
		return name[i+1:]
	}
	return name
}
