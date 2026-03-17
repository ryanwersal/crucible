package action

import (
	"fmt"

	"github.com/ryanwersal/crucible/internal/fact"
)

// DesiredPackage describes a Homebrew package that should be installed.
type DesiredPackage struct {
	Name string
	Type string // "formula" or "cask"
}

// DiffHomebrew compares desired packages against installed state.
func DiffHomebrew(desired []DesiredPackage, actual *fact.HomebrewInfo) ([]Action, error) {
	if !actual.Available {
		return nil, fmt.Errorf("homebrew is required but not installed")
	}

	var actions []Action
	for _, pkg := range desired {
		installed := false
		switch pkg.Type {
		case "formula":
			installed = actual.Formulae[pkg.Name]
		case "cask":
			installed = actual.Casks[pkg.Name]
		default:
			return nil, fmt.Errorf("unknown package type %q for %s", pkg.Type, pkg.Name)
		}

		if !installed {
			actions = append(actions, Action{
				Type:        InstallPackage,
				PackageName: pkg.Name,
				PackageType: pkg.Type,
				Description: fmt.Sprintf("brew install %s (%s)", pkg.Name, pkg.Type),
			})
		}
	}
	return actions, nil
}
