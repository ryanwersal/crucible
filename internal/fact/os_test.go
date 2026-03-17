package fact

import (
	"context"
	"runtime"
	"testing"
)

func TestOSCollector(t *testing.T) {
	t.Parallel()
	c := OSCollector{}
	info, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if info.OS != runtime.GOOS {
		t.Fatalf("OS mismatch: got %q, want %q", info.OS, runtime.GOOS)
	}
	if info.Arch != runtime.GOARCH {
		t.Fatalf("Arch mismatch: got %q, want %q", info.Arch, runtime.GOARCH)
	}
	if info.Hostname == "" {
		t.Fatal("expected non-empty hostname")
	}
}
