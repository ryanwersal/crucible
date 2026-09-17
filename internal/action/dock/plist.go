package dock

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"howett.net/plist"
)

// PlistState represents the dock layout and settings extracted from the plist.
type PlistState struct {
	Apps     []string      // app bundle paths from persistent-apps
	Folders  []FolderEntry // from persistent-others
	Settings Settings
}

// Settings holds Dock behavior keys. Nil pointers and a zero TileSize mean the
// key is absent from the plist.
type Settings struct {
	Autohide    *bool // autohide
	TileSize    int   // tilesize
	ShowRecents *bool // show-recents
}

// Layout describes the persistent-apps and persistent-others entries to write.
type Layout struct {
	Apps    []string
	Folders []FolderEntry
}

// FolderEntry describes a folder in the Dock's persistent-others.
type FolderEntry struct {
	Path    string
	View    string // "grid", "list", "fan", "auto" (maps to showas int)
	Display string // "folder", "stack" (maps to displayas int)
}

// Read parses a dock plist file and extracts the app and folder layout.
func Read(path string) (*PlistState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read dock plist: %w", err)
	}

	var root map[string]any
	if _, err := plist.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("unmarshal dock plist: %w", err)
	}

	state := &PlistState{}
	state.Apps = extractApps(root)
	state.Folders = extractFolders(root)
	state.Settings = extractSettings(root)
	return state, nil
}

func extractSettings(root map[string]any) Settings {
	var s Settings
	if v, ok := root["autohide"].(bool); ok {
		s.Autohide = &v
	}
	if v, ok := root["show-recents"].(bool); ok {
		s.ShowRecents = &v
	}
	s.TileSize = intValue(root["tilesize"])
	return s
}

func intValue(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case uint64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

// Write updates a dock plist file with the desired layout (when non-nil) and
// settings, preserving all other dock keys. Each layout entry includes a
// security-scoped bookmark, bundle identifier, and GUID so that macOS can
// resolve app icons.
func Write(path string, layout *Layout, settings Settings) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read dock plist: %w", err)
	}

	var root map[string]any
	if _, err := plist.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("unmarshal dock plist: %w", err)
	}

	if layout != nil {
		appEntries, err := buildAppEntries(layout.Apps)
		if err != nil {
			return fmt.Errorf("build app entries: %w", err)
		}
		folderEntries, err := buildFolderEntries(layout.Folders)
		if err != nil {
			return fmt.Errorf("build folder entries: %w", err)
		}

		root["persistent-apps"] = appEntries
		root["persistent-others"] = folderEntries
	}

	if settings.Autohide != nil {
		root["autohide"] = *settings.Autohide
	}
	if settings.TileSize > 0 {
		root["tilesize"] = uint64(settings.TileSize)
	}
	if settings.ShowRecents != nil {
		root["show-recents"] = *settings.ShowRecents
	}

	out, err := plist.Marshal(root, plist.BinaryFormat)
	if err != nil {
		return fmt.Errorf("marshal dock plist: %w", err)
	}

	if err := os.WriteFile(path, out, 0o600); err != nil {
		return fmt.Errorf("write dock plist: %w", err)
	}

	return nil
}

// RestartDock sends killall to restart the Dock process and apply changes.
func RestartDock(ctx context.Context) error {
	return exec.CommandContext(ctx, "killall", "Dock").Run()
}

func buildAppEntries(apps []string) ([]any, error) {
	entries := make([]any, len(apps))
	for i, app := range apps {
		bookmark, err := CreateBookmark(app)
		if err != nil {
			return nil, fmt.Errorf("bookmark %s: %w", app, err)
		}

		tileData := map[string]any{
			"file-data": map[string]any{
				"_CFURLString":     fileURL(app),
				"_CFURLStringType": uint64(15),
			},
			"file-label":      appLabel(app),
			"file-type":       uint64(41),
			"book":            bookmark,
			"dock-extra":      false,
			"is-beta":         false,
			"file-mod-date":   uint64(0),
			"parent-mod-date": uint64(0),
		}

		if bid := BundleIdentifier(app); bid != "" {
			tileData["bundle-identifier"] = bid
		}

		entries[i] = map[string]any{
			"GUID":      randGUID(),
			"tile-data": tileData,
			"tile-type": "file-tile",
		}
	}
	return entries, nil
}

func buildFolderEntries(folders []FolderEntry) ([]any, error) {
	entries := make([]any, len(folders))
	for i, f := range folders {
		bookmark, err := CreateBookmark(f.Path)
		if err != nil {
			return nil, fmt.Errorf("bookmark %s: %w", f.Path, err)
		}

		entries[i] = map[string]any{
			"GUID": randGUID(),
			"tile-data": map[string]any{
				"file-data": map[string]any{
					"_CFURLString":     fileURL(f.Path),
					"_CFURLStringType": uint64(15),
				},
				"file-label":        folderLabel(f.Path),
				"file-type":         uint64(2),
				"book":              bookmark,
				"showas":            viewToShowAs(f.View),
				"displayas":         displayToDisplayAs(f.Display),
				"arrangement":       uint64(1),
				"preferreditemsize": ^uint64(0),
			},
			"tile-type": "directory-tile",
		}
	}
	return entries, nil
}

// randGUID generates a random uint64 for dock entry GUIDs.
func randGUID() uint64 {
	var buf [8]byte
	_, _ = rand.Read(buf[:])
	return binary.LittleEndian.Uint64(buf[:])
}

func appLabel(path string) string {
	// "/Applications/Firefox.app" → "Firefox"
	base := path
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	return strings.TrimSuffix(base, ".app")
}

func folderLabel(path string) string {
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
}

func extractApps(root map[string]any) []string {
	apps, ok := root["persistent-apps"].([]any)
	if !ok {
		return nil
	}

	paths := make([]string, 0, len(apps))
	for _, item := range apps {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		tileData, ok := entry["tile-data"].(map[string]any)
		if !ok {
			continue
		}
		fileData, ok := tileData["file-data"].(map[string]any)
		if !ok {
			continue
		}
		urlStr, ok := fileData["_CFURLString"].(string)
		if !ok {
			continue
		}
		urlStr = pathFromFileURL(urlStr)
		paths = append(paths, urlStr)
	}
	return paths
}

func extractFolders(root map[string]any) []FolderEntry {
	others, ok := root["persistent-others"].([]any)
	if !ok {
		return nil
	}

	folders := make([]FolderEntry, 0, len(others))
	for _, item := range others {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		tileData, ok := entry["tile-data"].(map[string]any)
		if !ok {
			continue
		}
		fileData, ok := tileData["file-data"].(map[string]any)
		if !ok {
			continue
		}
		urlStr, ok := fileData["_CFURLString"].(string)
		if !ok {
			continue
		}
		urlStr = pathFromFileURL(urlStr)

		folder := FolderEntry{Path: urlStr}

		if showAs, ok := tileData["showas"].(uint64); ok {
			folder.View = showAsToView(showAs)
		}
		if displayAs, ok := tileData["displayas"].(uint64); ok {
			folder.Display = displayAsToDisplay(displayAs)
		}

		folders = append(folders, folder)
	}
	return folders
}

func viewToShowAs(view string) uint64 {
	switch view {
	case "fan":
		return 1
	case "grid":
		return 2
	case "list":
		return 3
	case "auto":
		return 0
	default:
		return 0
	}
}

func showAsToView(showAs uint64) string {
	switch showAs {
	case 1:
		return "fan"
	case 2:
		return "grid"
	case 3:
		return "list"
	default:
		return "auto"
	}
}

func displayToDisplayAs(display string) uint64 {
	switch display {
	case "folder":
		return 1
	case "stack":
		return 0
	default:
		return 0
	}
}

func displayAsToDisplay(displayAs uint64) string {
	switch displayAs {
	case 1:
		return "folder"
	default:
		return "stack"
	}
}

// fileURL renders a filesystem path as the percent-encoded file URL the Dock stores.
func fileURL(path string) string {
	u := url.URL{Scheme: "file", Path: path + "/"}
	return u.String()
}

// pathFromFileURL decodes a Dock file URL back to a filesystem path.
func pathFromFileURL(raw string) string {
	trimmed := strings.TrimPrefix(raw, "file://")
	trimmed = strings.TrimRight(trimmed, "/")
	if decoded, err := url.PathUnescape(trimmed); err == nil {
		return decoded
	}
	return trimmed
}
