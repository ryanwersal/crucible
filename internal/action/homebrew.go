package action

import (
	"fmt"
	"strings"

	"github.com/ryanwersal/crucible/internal/fact"
)

// DesiredPackage describes a Homebrew package that should be installed.
type DesiredPackage struct {
	Name string // may be a tap-qualified name like "owner/tap/formula"
}

// DiffHomebrew compares desired packages against installed state.
func DiffHomebrew(desired []DesiredPackage, actual *fact.HomebrewInfo) ([]Action, error) {
	if !actual.Available {
		return nil, fmt.Errorf("homebrew is required but not installed")
	}

	var actions []Action
	for _, pkg := range desired {
		if !isInstalled(pkg.Name, actual) {
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
// Tap-qualified names like "owner/tap/formula" are matched by their short name.
func isInstalled(name string, actual *fact.HomebrewInfo) bool {
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
