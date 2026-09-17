package action

import (
	"fmt"
	"slices"
	"strings"

	"github.com/ryanwersal/crucible/internal/fact"
)

// DockSettings holds optional Dock behavior settings. Nil pointers and zero
// values leave the corresponding setting unmanaged.
type DockSettings struct {
	Autohide    *bool
	TileSize    int
	ShowRecents *bool
}

// DesiredDock describes the desired macOS Dock layout and behavior.
type DesiredDock struct {
	Layout   bool // Apps and Folders are managed
	Apps     []string
	Folders  []DockFolder
	Settings DockSettings
}

// DiffDock compares the desired Dock state against the current state.
// The Dock is managed as a whole unit: any difference emits a single SetDock action.
func DiffDock(desired DesiredDock, actual *fact.DockInfo) []Action {
	if actual == nil {
		actual = &fact.DockInfo{}
	}

	var diffs []string

	if desired.Layout && !dockLayoutMatches(desired, actual) {
		diffs = append(diffs, "layout")
	}

	if want := desired.Settings.Autohide; want != nil && (actual.Autohide == nil || *want != *actual.Autohide) {
		diffs = append(diffs, fmt.Sprintf("autohide → %t", *want))
	}

	if want := desired.Settings.TileSize; want > 0 && want != actual.TileSize {
		diffs = append(diffs, fmt.Sprintf("tile size → %d", want))
	}

	if want := desired.Settings.ShowRecents; want != nil && (actual.ShowRecents == nil || *want != *actual.ShowRecents) {
		diffs = append(diffs, fmt.Sprintf("show recents → %t", *want))
	}

	if len(diffs) == 0 {
		return nil
	}

	return []Action{{
		Type:         SetDock,
		SerialGroup:  "dock",
		DockLayout:   desired.Layout,
		DockApps:     desired.Apps,
		DockFolders:  desired.Folders,
		DockSettings: desired.Settings,
		Description:  "set dock: " + strings.Join(diffs, ", "),
	}}
}

func dockLayoutMatches(desired DesiredDock, actual *fact.DockInfo) bool {
	if !slices.Equal(desired.Apps, actual.Apps) {
		return false
	}

	if len(desired.Folders) != len(actual.Folders) {
		return false
	}

	for i, df := range desired.Folders {
		af := actual.Folders[i]
		if df.Path != af.Path || df.View != af.View || df.Display != af.Display {
			return false
		}
	}

	return true
}
