package fact

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ryanwersal/crucible/internal/softwareupdate"
)

func TestSoftwareUpdateReceiptAndCatalogCollection(t *testing.T) {
	for _, latest := range []bool{false, true} {
		scans := 0
		marker := filepath.Join(t.TempDir(), "marker")
		client := &softwareupdate.Client{Marker: marker, CLTUsable: func() bool { return true }, Run: func(_ context.Context, name string, _ ...string) ([]byte, error) {
			if name == "/usr/sbin/pkgutil" {
				return []byte(softwareupdate.CLTReceipt + "\n"), nil
			}
			if _, err := os.Lstat(marker); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("installed CLT must not use the install-on-demand marker")
			}
			scans++
			return []byte("No new software available."), nil
		}}
		collector := SoftwareUpdateCollector{Client: client, Requests: []softwareupdate.Request{{Name: "CLT", Receipt: softwareupdate.CLTReceipt, CommandLineTools: true, Latest: latest}}}
		if _, err := collector.Collect(t.Context()); err != nil {
			t.Fatal(err)
		}
		expected := 0
		if latest {
			expected = 1
		}
		if scans != expected {
			t.Fatalf("latest %v scanned %d times", latest, scans)
		}
	}
}

func TestSoftwareUpdateReceiptFailureDoesNotBecomeMissing(t *testing.T) {
	sentinel := errors.New("inspection failed")
	client := &softwareupdate.Client{Run: func(context.Context, string, ...string) ([]byte, error) { return nil, sentinel }}
	_, err := (SoftwareUpdateCollector{Client: client}).Collect(t.Context())
	if !errors.Is(err, sentinel) {
		t.Fatalf("lost inspection failure: %v", err)
	}
}
