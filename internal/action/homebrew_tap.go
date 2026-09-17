package action

import (
	"fmt"
	"slices"
	"strings"

	"github.com/ryanwersal/crucible/internal/fact"
)

type DesiredHomebrewTap struct {
	Name   string
	Absent bool
}

// PackageTap returns the repository part of a qualified package name.
func PackageTap(name string) string {
	parts := strings.Split(name, "/")
	if len(parts) != 3 {
		return ""
	}
	return strings.ToLower(parts[0] + "/" + strings.TrimPrefix(parts[1], "homebrew-"))
}

func tapStates(desired []DesiredHomebrewTap) (map[string]bool, error) {
	states := make(map[string]bool, len(desired))
	for _, tap := range desired {
		if absent, exists := states[tap.Name]; exists && absent != tap.Absent {
			return nil, fmt.Errorf("conflicting states for Homebrew tap %s", tap.Name)
		}
		states[tap.Name] = tap.Absent
	}
	return states, nil
}

// ValidateHomebrewTaps refuses implicit tap creation and removal of repositories
// still needed by managed or installed packages. The latter check includes
// packages outside the script: removing a tap must never remove those apps.
func ValidateHomebrewTaps(taps []DesiredHomebrewTap, packages []DesiredPackage, actual *fact.HomebrewTapInfo, brew *fact.HomebrewInfo) error {
	states, err := tapStates(taps)
	if err != nil {
		return err
	}
	for _, pkg := range packages {
		tap := PackageTap(pkg.Name)
		if tap == "" {
			for _, installed := range brew.InstalledPackages {
				if matchesHomebrewPackage(pkg, installed) {
					tap = installed.Tap
					break
				}
			}
		}
		if pkg.Absent {
			continue
		}
		absent, declared := states[tap]
		if declared && absent {
			return fmt.Errorf("package %s requires tap %s, which is declared absent", pkg.Name, tap)
		}
		if tap != "" && tap != "homebrew/core" && tap != "homebrew/cask" && !actual.Taps[tap] && !declared {
			return fmt.Errorf("package %s requires missing tap %s; declare c.brew.tap(%q) explicitly", pkg.Name, tap, tap)
		}
	}
	for tap, absent := range states {
		if !absent {
			continue
		}
		for _, installed := range brew.InstalledPackages {
			if installed.Tap != tap {
				continue
			}
			removed := false
			for _, pkg := range packages {
				if pkg.Absent && matchesHomebrewPackage(pkg, installed) {
					removed = true
					break
				}
			}
			if !removed {
				return fmt.Errorf("cannot remove tap %s: installed %s %s must be declared absent first", tap, installed.Kind, installed.Name)
			}
		}
	}
	return nil
}

func DiffHomebrewTaps(desired []DesiredHomebrewTap, actual *fact.HomebrewTapInfo) ([]Action, []Observation, error) {
	if !actual.Available {
		return nil, nil, fmt.Errorf("homebrew is required but not installed")
	}
	states, err := tapStates(desired)
	if err != nil {
		return nil, nil, err
	}
	var actions []Action
	var observations []Observation
	seen := make(map[string]bool, len(states))
	for _, tap := range desired {
		if seen[tap.Name] {
			continue
		}
		seen[tap.Name] = true
		installed := actual.Taps[tap.Name]
		switch {
		case tap.Absent && installed:
			actions = append(actions, Action{Type: RemoveHomebrewTap, TapName: tap.Name, SerialGroup: homebrewSerialGroup, Description: "brew untap " + tap.Name})
		case !tap.Absent && !installed:
			actions = append(actions, Action{Type: AddHomebrewTap, TapName: tap.Name, SerialGroup: homebrewSerialGroup, Description: "brew tap " + tap.Name})
		default:
			state := "tapped"
			if tap.Absent {
				state = "already absent"
			}
			observations = append(observations, Observation{Description: fmt.Sprintf("%s (%s)", tap.Name, state)})
		}
	}
	return actions, observations, nil
}

// OrderHomebrewActions orders only the Homebrew slots, preserving the placement
// of unrelated resources and package declaration order within each phase.
func OrderHomebrewActions(actions []Action) {
	var positions []int
	var brew []Action
	for i, a := range actions {
		if a.SerialGroup == homebrewSerialGroup {
			positions = append(positions, i)
			brew = append(brew, a)
		}
	}
	phase := func(a Action) int {
		switch a.Type {
		case AddHomebrewTap:
			return 0
		case RemoveHomebrewTap:
			return 2
		default:
			return 1
		}
	}
	slices.SortStableFunc(brew, func(a, b Action) int { return phase(a) - phase(b) })
	for i, position := range positions {
		actions[position] = brew[i]
	}
}
