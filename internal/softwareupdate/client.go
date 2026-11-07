package softwareupdate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/semaphore"
)

const CLTReceipt = "com.apple.pkg.CLTools_Executables"
const discoveryMarker = "/tmp/.com.apple.dt.CommandLineTools.installondemand.in-progress"

// Client's read operations are injectable; installation remains in the resource
// executor so the existing privilege and progress handling is retained.
type Client struct {
	Run       func(context.Context, string, ...string) ([]byte, error)
	Marker    string
	CLTUsable func() bool
}

func NewClient() *Client {
	return &Client{Run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if runtime.GOOS != "darwin" {
			return nil, fmt.Errorf("softwareupdate requires macOS")
		}
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
		output, err := cmd.CombinedOutput()
		if err != nil {
			if ctx.Err() != nil {
				err = errors.Join(err, ctx.Err())
			}
			return nil, fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(string(output)))
		}
		return output, nil
	}, Marker: discoveryMarker, CLTUsable: func() bool {
		clang, err := os.Stat("/Library/Developer/CommandLineTools/usr/bin/clang")
		if err != nil || !clang.Mode().IsRegular() || clang.Mode().Perm()&0o111 == 0 {
			return false
		}
		sdk, err := os.Stat("/Library/Developer/CommandLineTools/SDKs/MacOSX.sdk")
		return err == nil && sdk.IsDir()
	}}
}

func (c *Client) Receipts(ctx context.Context) (map[string]bool, error) {
	output, err := c.Run(ctx, "/usr/sbin/pkgutil", "--pkgs")
	if err != nil {
		return nil, fmt.Errorf("inspect installed package receipts: %w", err)
	}
	receipts := make(map[string]bool)
	for _, name := range strings.Fields(string(output)) {
		receipts[name] = true
	}
	return receipts, nil
}

var scanLock = semaphore.NewWeighted(1)

func (c *Client) Catalog(ctx context.Context, discoverCLT bool) (updates []Update, err error) {
	if err := scanLock.Acquire(ctx, 1); err != nil {
		return nil, err
	}
	defer scanLock.Release(1)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// The marker changes discovery, not installed state. Preserve existing markers;
	// remove only the exact regular file this scan created, even on cancellation.
	if discoverCLT {
		cleanup, createErr := createMarker(c.Marker)
		if createErr != nil {
			return nil, createErr
		}
		defer func() { err = errors.Join(err, cleanup()) }()
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	output, err := c.Run(ctx, "/usr/sbin/softwareupdate", "--list")
	if err != nil {
		return nil, fmt.Errorf("scan Apple updates: %w", err)
	}
	return ParseCatalog(string(output))
}

func createMarker(path string) (func() error, error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		info, statErr := os.Lstat(path)
		if statErr != nil {
			return nil, statErr
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("CLT discovery marker is not a regular file: %s", path)
		}
		return func() error { return nil }, nil
	}
	if err != nil {
		return nil, fmt.Errorf("create CLT discovery marker: %w", err)
	}
	original, statErr := file.Stat()
	closeErr := file.Close()
	if err := errors.Join(statErr, closeErr); err != nil {
		return nil, err
	}
	return sync.OnceValue(func() error {
		current, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if !os.SameFile(original, current) {
			return fmt.Errorf("CLT discovery marker changed during scan; left untouched")
		}
		return os.Remove(path)
	}), nil
}

// PrepareDiscovery keeps the CLT offer discoverable during installation as well
// as catalog scans. The caller must run cleanup on every return path.
func (c *Client) PrepareDiscovery(enabled bool) (func() error, error) {
	if !enabled {
		return func() error { return nil }, nil
	}
	return createMarker(c.Marker)
}
