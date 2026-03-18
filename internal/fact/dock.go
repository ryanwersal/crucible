package fact

import (
	"context"
	"path/filepath"

	"github.com/ryanwersal/crucible/internal/action/dock"
)

// DockInfo holds the current macOS Dock layout.
type DockInfo struct {
	Apps    []string
	Folders []DockFolderInfo
}

// DockFolderInfo describes a folder entry in the Dock.
type DockFolderInfo struct {
	Path    string
	View    string
	Display string
}

// DockCollector reads the current Dock plist.
type DockCollector struct {
	HomeDir string
}

// Collect reads the Dock plist and extracts the current layout.
func (c DockCollector) Collect(ctx context.Context) (*DockInfo, error) {
	plistPath := filepath.Join(c.HomeDir, "Library", "Preferences", "com.apple.dock.plist")
	state, err := dock.Read(plistPath)
	if err != nil {
		return nil, err
	}

	info := &DockInfo{Apps: state.Apps}
	for _, f := range state.Folders {
		info.Folders = append(info.Folders, DockFolderInfo{
			Path:    f.Path,
			View:    f.View,
			Display: f.Display,
		})
	}

	return info, nil
}
