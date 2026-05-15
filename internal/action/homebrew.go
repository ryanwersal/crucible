package action

import (
	"fmt"
	"strings"

	"github.com/ryanwersal/crucible/internal/fact"
)

// DesiredPackage describes a Homebrew package that should be installed,
// removed, or kept up to date.
type DesiredPackage struct {
	Name   string // may be a tap-qualified name like "owner/tap/formula"
	Absent bool   // true = ensure the package is not installed
	Latest bool   // true = ensure installed AND at the current version
}

// DiffHomebrew compares desired packages against installed state. Returns
// the actions needed to converge, any informational observations (e.g.
// "package pinned, skipping upgrade"), and the set of package names the
// diff produced an action or observation for — callers use that set to
// suppress duplicate "(installed)" / "(already absent)" notes.
func DiffHomebrew(desired []DesiredPackage, actual *fact.HomebrewInfo) ([]Action, []Observation, map[string]bool, error) {
	if !actual.Available {
		return nil, nil, nil, fmt.Errorf("homebrew is required but not installed")
	}

	var actions []Action
	var obs []Observation
	noted := make(map[string]bool)

	for _, pkg := range desired {
		installed := isInstalled(pkg.Name, actual)

		if pkg.Absent {
			if installed {
				actions = append(actions, Action{
					Type:        UninstallPackage,
					PackageName: pkg.Name,
					Description: fmt.Sprintf("brew uninstall %s", pkg.Name),
				})
				noted[pkg.Name] = true
			}
			continue
		}

		if !installed {
			actions = append(actions, Action{
				Type:        InstallPackage,
				PackageName: pkg.Name,
				Description: fmt.Sprintf("brew install %s", pkg.Name),
			})
			noted[pkg.Name] = true
			continue
		}

		if !pkg.Latest {
			continue
		}

		out, isOutdated := lookupOutdated(pkg.Name, actual)
		if !isOutdated {
			continue
		}
		if out.Pinned {
			obs = append(obs, Observation{
				Description: fmt.Sprintf("%s (pinned, skipping upgrade to %s)", pkg.Name, out.CurrentVersion),
			})
			noted[pkg.Name] = true
			continue
		}
		if out.AutoUpdates {
			obs = append(obs, Observation{
				Description: fmt.Sprintf("%s (auto-updates outside of brew, skipping upgrade)", pkg.Name),
			})
			noted[pkg.Name] = true
			continue
		}

		actions = append(actions, Action{
			Type:                    UpgradePackage,
			PackageName:             pkg.Name,
			PackageInstalledVersion: out.InstalledVersion,
			PackageCurrentVersion:   out.CurrentVersion,
			Description: fmt.Sprintf("brew upgrade %s (%s → %s)",
				pkg.Name, out.InstalledVersion, out.CurrentVersion),
		})
		noted[pkg.Name] = true
	}
	return actions, obs, noted, nil
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

// lookupOutdated returns the outdated entry for a package, if any. It tries
// the user-provided name first, then the tap-qualified short name, mirroring
// the resolution that isInstalled does.
func lookupOutdated(name string, actual *fact.HomebrewInfo) (fact.OutdatedPackage, bool) {
	if p, ok := actual.Outdated[name]; ok {
		return p, true
	}
	short := shortName(name)
	p, ok := actual.Outdated[short]
	return p, ok
}

// shortName extracts the trailing formula/cask name from a tap-qualified
// package name. "owner/tap/formula" → "formula". Plain names are unchanged.
func shortName(name string) string {
	if i := strings.LastIndex(name, "/"); i >= 0 {
		return name[i+1:]
	}
	return name
}
