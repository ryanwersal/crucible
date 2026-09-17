package action

import (
	"fmt"
	"slices"
	"strings"

	"github.com/ryanwersal/crucible/internal/fact"
)

// homebrewSerialGroup is the SerialGroup tag for all brew install/uninstall/
// upgrade actions. Brew operations contend on shared Cellar paths (readline,
// ca-certificates, and other common deps) and corrupt their lockfiles when
// run concurrently — so they all chain together regardless of package name.
const homebrewSerialGroup = "homebrew"

// DesiredPackage describes a Homebrew package that should be installed,
// removed, or kept up to date.
type DesiredPackage struct {
	Kind   string // empty (auto), formula, or cask
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
		installed := packageInstalled(pkg, actual)

		if pkg.Absent {
			if installed {
				actions = append(actions, Action{
					Type:        UninstallPackage,
					PackageName: pkg.Name,
					PackageKind: pkg.Kind,
					Description: fmt.Sprintf("brew uninstall %s", pkg.Name),
					SerialGroup: homebrewSerialGroup,
				})
				noted[pkg.Name] = true
			}
			continue
		}

		if !installed {
			actions = append(actions, Action{
				Type:        InstallPackage,
				PackageName: pkg.Name,
				PackageKind: pkg.Kind,
				Description: fmt.Sprintf("brew install %s", pkg.Name),
				SerialGroup: homebrewSerialGroup,
			})
			noted[pkg.Name] = true
			continue
		}

		if !pkg.Latest {
			continue
		}

		out, isOutdated := lookupOutdated(pkg.Name, actual)
		if pkg.Kind != "" {
			if typed, ok := actual.Outdated[pkg.Kind+":"+pkg.Name]; ok {
				out, isOutdated = typed, true
			}
			if isOutdated && out.IsCask != (pkg.Kind == "cask") {
				isOutdated = false
			}
		}
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
			PackageKind:             pkg.Kind,
			PackageInstalledVersion: out.InstalledVersion,
			PackageCurrentVersion:   out.CurrentVersion,
			Description: fmt.Sprintf("brew upgrade %s (%s → %s)",
				pkg.Name, out.InstalledVersion, out.CurrentVersion),
			SerialGroup: homebrewSerialGroup,
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

func matchesHomebrewPackage(desired DesiredPackage, installed fact.HomebrewPackage) bool {
	if desired.Kind != "" && desired.Kind != installed.Kind {
		return false
	}
	if tap := PackageTap(desired.Name); tap != "" && tap != installed.Tap {
		return false
	}
	return slices.Contains(installed.Names, desired.Name) || (PackageTap(desired.Name) != "" && installed.Name == shortName(desired.Name))
}

func packageInstalled(pkg DesiredPackage, actual *fact.HomebrewInfo) bool {
	if actual.InstalledPackages != nil {
		for _, installed := range actual.InstalledPackages {
			if matchesHomebrewPackage(pkg, installed) {
				return true
			}
		}
		return false
	}
	switch pkg.Kind {
	case "formula":
		return actual.Formulae[pkg.Name] || actual.Formulae[shortName(pkg.Name)]
	case "cask":
		return actual.Casks[pkg.Name] || actual.Casks[shortName(pkg.Name)]
	default:
		return isInstalled(pkg.Name, actual)
	}
}
