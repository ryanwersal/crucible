package fact

import (
	"context"
	"os/user"
	"runtime"
	"testing"
)

func TestShellCollector_Collect(t *testing.T) {
	t.Parallel()

	if runtime.GOOS != "darwin" {
		t.Skip("ShellCollector uses dscl, macOS only")
	}

	u, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}

	c := ShellCollector{Username: u.Username}
	info, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// Path may be empty if dscl has no shell entry (e.g. CI runner accounts).
	// When set, it must be an absolute path.
	if info.Path != "" && info.Path[0] != '/' {
		t.Errorf("shell path %q doesn't start with /", info.Path)
	}
}

func TestShellCollector_DefaultUsername(t *testing.T) {
	t.Parallel()

	if runtime.GOOS != "darwin" {
		t.Skip("ShellCollector uses dscl, macOS only")
	}

	c := ShellCollector{} // empty username should use current user
	info, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// Path may be empty if dscl has no shell entry for the current user.
	if info.Path != "" && info.Path[0] != '/' {
		t.Errorf("shell path %q doesn't start with /", info.Path)
	}
}
