package dock

import (
	"os"
	"path/filepath"
	"testing"

	"howett.net/plist"
)

func TestReadWrite_RoundTrip(t *testing.T) {
	t.Parallel()

	// Build a minimal dock plist in XML format
	root := map[string]any{
		"autohide": true,
		"tilesize": uint64(36),
		"persistent-apps": []any{
			map[string]any{
				"tile-data": map[string]any{
					"file-data": map[string]any{
						"_CFURLString":     "file:///Applications/Safari.app/",
						"_CFURLStringType": uint64(15),
					},
					"file-label":        "Safari",
					"file-type":         uint64(41),
					"bundle-identifier": "com.apple.Safari",
				},
				"tile-type": "file-tile",
			},
			map[string]any{
				"tile-data": map[string]any{
					"file-data": map[string]any{
						"_CFURLString":     "file:///System/Applications/Utilities/Terminal.app/",
						"_CFURLStringType": uint64(15),
					},
					"file-label": "Terminal",
					"file-type":  uint64(41),
				},
				"tile-type": "file-tile",
			},
		},
		"persistent-others": []any{
			map[string]any{
				"tile-data": map[string]any{
					"file-data": map[string]any{
						"_CFURLString":     "file:///Users/test/Downloads/",
						"_CFURLStringType": uint64(15),
					},
					"file-label":        "Downloads",
					"file-type":         uint64(2),
					"showas":            uint64(2), // grid
					"displayas":         uint64(1), // folder
					"arrangement":       uint64(1),
					"preferreditemsize": uint64(0),
				},
				"tile-type": "directory-tile",
			},
		},
	}

	data, err := plist.Marshal(root, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	plistPath := filepath.Join(dir, "com.apple.dock.plist")
	if err := os.WriteFile(plistPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	// Read
	state, err := Read(plistPath)
	if err != nil {
		t.Fatal(err)
	}

	if len(state.Apps) != 2 {
		t.Fatalf("expected 2 apps, got %d", len(state.Apps))
	}
	if state.Apps[0] != "/Applications/Safari.app" {
		t.Fatalf("expected Safari, got %q", state.Apps[0])
	}
	if state.Apps[1] != "/System/Applications/Utilities/Terminal.app" {
		t.Fatalf("expected Terminal, got %q", state.Apps[1])
	}

	if len(state.Folders) != 1 {
		t.Fatalf("expected 1 folder, got %d", len(state.Folders))
	}
	if state.Folders[0].Path != "/Users/test/Downloads" {
		t.Fatalf("expected Downloads path, got %q", state.Folders[0].Path)
	}
	if state.Folders[0].View != "grid" {
		t.Fatalf("expected grid view, got %q", state.Folders[0].View)
	}
	if state.Folders[0].Display != "folder" {
		t.Fatalf("expected folder display, got %q", state.Folders[0].Display)
	}

	// Write with apps that exist on this system so bookmarks can be created
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	newApps := []string{"/System/Applications/Utilities/Terminal.app"}
	newFolders := []FolderEntry{{Path: homeDir, View: "list", Display: "stack"}}
	if err := Write(plistPath, newApps, newFolders); err != nil {
		t.Fatal(err)
	}

	// Read back
	state2, err := Read(plistPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(state2.Apps) != 1 {
		t.Fatalf("expected 1 app after write, got %d", len(state2.Apps))
	}
	if state2.Apps[0] != "/System/Applications/Utilities/Terminal.app" {
		t.Fatalf("expected Terminal, got %q", state2.Apps[0])
	}
	if len(state2.Folders) != 1 {
		t.Fatalf("expected 1 folder after write, got %d", len(state2.Folders))
	}
	if state2.Folders[0].View != "list" {
		t.Fatalf("expected list view, got %q", state2.Folders[0].View)
	}

	// Verify other settings preserved
	readData, err := os.ReadFile(plistPath)
	if err != nil {
		t.Fatal(err)
	}
	var readRoot map[string]any
	if _, err := plist.Unmarshal(readData, &readRoot); err != nil {
		t.Fatal(err)
	}
	if ah, ok := readRoot["autohide"].(bool); !ok || !ah {
		t.Fatal("autohide setting not preserved")
	}

	// Verify bookmark data was generated
	apps, ok := readRoot["persistent-apps"].([]any)
	if !ok || len(apps) == 0 {
		t.Fatal("no apps in written plist")
	}
	appEntry, ok := apps[0].(map[string]any)
	if !ok {
		t.Fatal("app entry is not a map")
	}
	tileData, ok := appEntry["tile-data"].(map[string]any)
	if !ok {
		t.Fatal("tile-data is not a map")
	}
	if _, ok := tileData["book"]; !ok {
		t.Error("missing book field in written app entry")
	}
	if _, ok := tileData["bundle-identifier"]; !ok {
		t.Error("missing bundle-identifier in written app entry")
	}
	if _, ok := appEntry["GUID"]; !ok {
		t.Error("missing GUID in written app entry")
	}
	fileData, ok := tileData["file-data"].(map[string]any)
	if !ok {
		t.Fatal("file-data is not a map")
	}
	if urlType, ok := fileData["_CFURLStringType"].(uint64); !ok || urlType != 15 {
		t.Errorf("_CFURLStringType = %v, want 15", fileData["_CFURLStringType"])
	}
}

func TestAppLabel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path string
		want string
	}{
		{"/Applications/Firefox.app", "Firefox"},
		{"/System/Applications/Launchpad.app", "Launchpad"},
		{"/Applications/Some App.app", "Some App"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := appLabel(tt.path)
			if got != tt.want {
				t.Errorf("appLabel(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestViewConversions(t *testing.T) {
	t.Parallel()
	for _, v := range []string{"fan", "grid", "list", "auto"} {
		got := showAsToView(viewToShowAs(v))
		if got != v {
			t.Errorf("roundtrip %q → %q", v, got)
		}
	}
}

func TestDisplayConversions(t *testing.T) {
	t.Parallel()
	for _, d := range []string{"folder", "stack"} {
		got := displayAsToDisplay(displayToDisplayAs(d))
		if got != d {
			t.Errorf("roundtrip %q → %q", d, got)
		}
	}
}
