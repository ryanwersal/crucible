package fact

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// DisplayInfo holds the current display-related system state.
type DisplayInfo struct {
	SidebarIconSize int    // NSTableViewDefaultSizeMode: 1=small, 2=medium, 3=large; 0 if unset
	MenuBarSpacing  int    // NSStatusItemSpacing; -1 if unset
	MenuBarPadding  int    // NSStatusItemSelectionPadding; -1 if unset
	Resolution      string // current built-in display resolution "WxH", empty if unknown
	HZ              int    // current refresh rate, 0 if unknown
}

// DisplayCollector reads current display density settings.
type DisplayCollector struct{}

// Collect gathers display density state from macOS defaults and CoreGraphics.
func (DisplayCollector) Collect(ctx context.Context) (*DisplayInfo, error) {
	info := &DisplayInfo{
		MenuBarSpacing: -1,
		MenuBarPadding: -1,
	}

	// Read sidebar icon size from NSGlobalDomain.
	if out, err := exec.CommandContext(ctx, "defaults", "read", "NSGlobalDomain", "NSTableViewDefaultSizeMode").Output(); err == nil {
		s := strings.TrimSpace(string(out))
		if v, err := strconv.Atoi(s); err == nil {
			info.SidebarIconSize = v
		}
	}

	// Read menu bar spacing from currentHost NSGlobalDomain.
	if out, err := exec.CommandContext(ctx, "defaults", "-currentHost", "read", "-globalDomain", "NSStatusItemSpacing").Output(); err == nil {
		s := strings.TrimSpace(string(out))
		if v, err := strconv.Atoi(s); err == nil {
			info.MenuBarSpacing = v
		}
	}

	if out, err := exec.CommandContext(ctx, "defaults", "-currentHost", "read", "-globalDomain", "NSStatusItemSelectionPadding").Output(); err == nil {
		s := strings.TrimSpace(string(out))
		if v, err := strconv.Atoi(s); err == nil {
			info.MenuBarPadding = v
		}
	}

	// Read current resolution from CoreGraphics.
	w, h, hz := builtInDisplayMode()
	if w > 0 && h > 0 {
		info.Resolution = fmt.Sprintf("%dx%d", w, h)
		info.HZ = hz
	}

	return info, nil
}

// SetBuiltInDisplayMode applies a display mode to the built-in display using CoreGraphics.
// The resolution string should be in "WxH" format. Hz of 0 means pick the best available rate.
func SetBuiltInDisplayMode(resolution string, hz int) error {
	w, h, err := parseResolution(resolution)
	if err != nil {
		return err
	}
	return setDisplayMode(w, h, hz)
}

// parseResolution splits "WxH" into width and height integers.
func parseResolution(res string) (int, int, error) {
	parts := strings.SplitN(res, "x", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid resolution format %q, expected WxH", res)
	}
	w, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid resolution width %q: %w", parts[0], err)
	}
	h, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid resolution height %q: %w", parts[1], err)
	}
	return w, h, nil
}
