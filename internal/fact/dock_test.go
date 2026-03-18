package fact

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"howett.net/plist"
)

func TestDockCollector_NonExistentPath(t *testing.T) {
	t.Parallel()
	c := DockCollector{HomeDir: "/nonexistent"}
	_, err := c.Collect(context.Background())
	if err == nil {
		t.Fatal("expected error for non-existent plist")
	}
}

func TestDockCollector_ValidPlist(t *testing.T) {
	t.Parallel()

	// Build a minimal dock plist
	root := map[string]any{
		"persistent-apps": []any{
			map[string]any{
				"tile-data": map[string]any{
					"file-data": map[string]any{
						"_CFURLString":     "file:///Applications/Safari.app/",
						"_CFURLStringType": uint64(0),
					},
					"file-label": "Safari",
					"file-type":  uint64(41),
				},
				"tile-type": "file-tile",
			},
		},
		"persistent-others": []any{},
	}

	data, err := plist.Marshal(root, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	prefDir := filepath.Join(dir, "Library", "Preferences")
	if err := os.MkdirAll(prefDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(prefDir, "com.apple.dock.plist"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	c := DockCollector{HomeDir: dir}
	info, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(info.Apps))
	}
	if info.Apps[0] != "/Applications/Safari.app" {
		t.Fatalf("expected Safari, got %q", info.Apps[0])
	}
}
