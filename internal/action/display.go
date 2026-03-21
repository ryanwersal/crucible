package action

import (
	"fmt"
	"strings"

	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/script/decl"
)

// DesiredDisplay describes the desired display density configuration.
type DesiredDisplay struct {
	SidebarIconSize string // "small", "medium", "large", or "" (unchanged)
	MenuBarSpacing  string // "compact", "default", or "" (unchanged)
	Resolution      string // "WxH" or "" (unchanged)
	HZ              int    // refresh rate, 0 = auto
}

// DiffDisplay compares desired display density settings against current state.
// Returns a single SetDisplay action if anything differs, nil if already up to date.
func DiffDisplay(desired DesiredDisplay, actual *fact.DisplayInfo) []Action {
	if actual == nil {
		actual = &fact.DisplayInfo{MenuBarSpacing: -1, MenuBarPadding: -1}
	}

	var diffs []string

	// Check sidebar icon size.
	if desired.SidebarIconSize != "" {
		wantVal, ok := decl.SidebarIconSizeValue(desired.SidebarIconSize)
		if !ok {
			return nil
		}
		if wantVal != actual.SidebarIconSize {
			diffs = append(diffs, fmt.Sprintf("sidebar icons → %s", desired.SidebarIconSize))
		}
	}

	// Check menu bar spacing.
	switch desired.MenuBarSpacing {
	case "compact":
		// Compact: NSStatusItemSpacing=6, NSStatusItemSelectionPadding=4
		if actual.MenuBarSpacing != 6 || actual.MenuBarPadding != 4 {
			diffs = append(diffs, "menu bar spacing → compact")
		}
	case "default":
		// Default: keys should not exist (values are -1 when unset)
		if actual.MenuBarSpacing != -1 || actual.MenuBarPadding != -1 {
			diffs = append(diffs, "menu bar spacing → default")
		}
	}

	// Check resolution.
	if desired.Resolution != "" {
		resMatch := desired.Resolution == actual.Resolution
		hzMatch := desired.HZ == 0 || desired.HZ == actual.HZ
		if !resMatch || !hzMatch {
			desc := fmt.Sprintf("resolution → %s", desired.Resolution)
			if desired.HZ > 0 {
				desc += fmt.Sprintf("@%dHz", desired.HZ)
			}
			diffs = append(diffs, desc)
		}
	}

	if len(diffs) == 0 {
		return nil
	}

	return []Action{{
		Type:                   SetDisplay,
		DisplaySidebarIconSize: desired.SidebarIconSize,
		DisplayMenuBarSpacing:  desired.MenuBarSpacing,
		DisplayResolution:      desired.Resolution,
		DisplayHZ:              desired.HZ,
		Description:            fmt.Sprintf("set display density: %s", strings.Join(diffs, ", ")),
	}}
}
