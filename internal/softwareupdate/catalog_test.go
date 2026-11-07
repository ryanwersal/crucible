package softwareupdate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

const catalog = `Software Update Tool
Finding available software
Software Update found the following new or updated software:
* Label: Command Line Tools for Xcode-26.9
 Title: Command Line Tools for Xcode, Version: 26.9, Size: 100KiB, Recommended: YES,
* Label: Command Line Tools for Xcode-26.10
 Title: Command Line Tools for Xcode, Version: 26.10, Size: 100KiB, Recommended: YES,
* Label: Safari26.1-26.1
 Title: Safari, Version: 26.1, Size: 100KiB, Recommended: YES,
* Label: macOS Tahoe 26.1-25B78
 Title: macOS Tahoe 26.1, Version: 26.1, Size: 100KiB, Recommended: YES, Action: restart,
`

func TestCatalogSelection(t *testing.T) {
	t.Parallel()
	updates, err := ParseCatalog(catalog)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, match, label string
		restart            bool
	}{
		{"Command Line Tools for Xcode", "prefix", "Command Line Tools for Xcode-26.10", false},
		{"Safari", "title", "Safari26.1-26.1", false},
		{"macOS Tahoe", "prefix", "macOS Tahoe 26.1-25B78", true},
		{"Command Line Tools for Xcode-26.9", "label", "Command Line Tools for Xcode-26.9", false},
	} {
		t.Run(tc.name+tc.match, func(t *testing.T) {
			got, err := (Request{Name: tc.name, Match: tc.match}).Select(updates)
			if err != nil || got == nil || got.Label != tc.label || got.Restart != tc.restart {
				t.Fatalf("got %+v, %v", got, err)
			}
		})
	}
	if got, err := (Request{Name: "Safari", Match: "title"}).Select(updates[:2]); err != nil || got != nil {
		t.Fatalf("unmatched: %+v %v", got, err)
	}
}

func TestCatalogRejectsMalformedOutput(t *testing.T) {
	t.Parallel()
	for _, output := range []string{"", "Network failed", "* Label: Test\nTitle: Test, Version: 1, Size: 1KiB, Action: unknown,", "* Label: Test", "* Label: --all\nTitle: Test, Version: 1, Size: 1KiB", "Title: Test, Version: 1, Size: 1KiB", "* Label: Test\nTitle: Test, Version: 1"} {
		if _, err := ParseCatalog(output); err == nil {
			t.Errorf("accepted %q", output)
		}
	}
	if updates, err := ParseCatalog("Software Update Tool\nNo new software available."); err != nil || len(updates) != 0 {
		t.Fatalf("empty catalog: %v %v", updates, err)
	}
}

func TestAmbiguousVersions(t *testing.T) {
	t.Parallel()
	request := Request{Name: "Test", Match: "title"}
	for _, versions := range [][2]string{{"1.0", "1"}, {"1.0b1", "1.0b2"}} {
		_, err := request.Select([]Update{{Label: "a", Title: "Test", Version: versions[0]}, {Label: "b", Title: "Test", Version: versions[1]}})
		if err == nil {
			t.Fatalf("accepted ambiguous versions %v", versions)
		}
	}
}

func TestMarkerLifecycle(t *testing.T) {
	for _, failure := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failure"}[failure], func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "marker")
			client := &Client{Marker: marker, Run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				if _, err := os.Stat(marker); err != nil {
					t.Fatal("marker not present during discovery")
				}
				if failure {
					return nil, context.Canceled
				}
				return []byte("No new software available."), nil
			}}
			_, err := client.Catalog(t.Context(), true)
			if failure && !errors.Is(err, context.Canceled) {
				t.Fatalf("lost error: %v", err)
			}
			if !failure && err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(marker); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("marker not removed: %v", err)
			}
		})
	}
}

func TestPreexistingMarkersArePreservedAndSymlinksRefused(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "marker")
	if err := os.WriteFile(path, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	cleanup, err := createMarker(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "existing" {
		t.Fatal("existing marker changed")
	}
	link := path + "-link"
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := createMarker(link); err == nil {
		t.Fatal("accepted symlink")
	}
}

func TestCanceledScanDoesNotWaitForAnotherScan(t *testing.T) {
	if err := scanLock.Acquire(t.Context(), 1); err != nil {
		t.Fatal(err)
	}
	defer scanLock.Release(1)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	client := &Client{}
	if _, err := client.Catalog(ctx, false); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}
