package fact

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"howett.net/plist"
)

// MacOSInfo holds the observed state of the tweaks managed by c.macos().
type MacOSInfo struct {
	// SpotlightIndexing is true if any eligible volume has indexing enabled.
	SpotlightIndexing      bool
	SpotlightIndexingMixed bool
	// SpotlightVolumes contains explicit eligible mount points. APFS Backup
	// volumes require indexing for Time Machine and are reported separately.
	SpotlightVolumes  []string
	SpotlightExcluded []string
	// MusicPlayKey records launchd's persistent enabled state separately from
	// whether the service is loaded, so interrupted changes can be reconciled.
	MusicPlayKey                bool
	MusicPlayKeyLoaded          bool
	ClickWallpaperToShowDesktop bool
}

// MacOSCollector reads only the requested tweaks.
type MacOSCollector struct {
	SpotlightIndexing           bool
	MusicPlayKey                bool
	ClickWallpaperToShowDesktop bool
}

func (c MacOSCollector) Collect(ctx context.Context) (*MacOSInfo, error) {
	info := &MacOSInfo{}
	var err error
	if c.SpotlightIndexing {
		err = collectSpotlightIndexing(ctx, info)
		if err != nil {
			return nil, err
		}
	}
	if c.MusicPlayKey {
		info.MusicPlayKey, err = remoteControlDaemonEnabled(ctx)
		if err != nil {
			return nil, err
		}
		info.MusicPlayKeyLoaded, err = LaunchdServiceLoaded(ctx, LaunchdUserDomain(), RemoteControlDaemonService)
		if err != nil {
			return nil, err
		}
	}
	if c.ClickWallpaperToShowDesktop {
		info.ClickWallpaperToShowDesktop, err = clickWallpaperToShowDesktopEnabled(ctx)
		if err != nil {
			return nil, err
		}
	}
	return info, nil
}

type spotlightVolume struct {
	path     string
	enabled  bool
	hasIndex bool
}

func collectSpotlightIndexing(ctx context.Context, info *MacOSInfo) error {
	out, err := exec.CommandContext(ctx, "mdutil", "-a", "-s").CombinedOutput()
	if err != nil {
		return fmt.Errorf("query Spotlight indexing state: %w: %s", err, strings.TrimSpace(string(out)))
	}
	// When the Spotlight server is disabled, -a can succeed with no output.
	// Query the root explicitly; never infer a disabled state from silence.
	if strings.TrimSpace(string(out)) == "" {
		out, err = exec.CommandContext(ctx, "mdutil", "-s", "/").CombinedOutput()
		if err != nil {
			return fmt.Errorf("query root Spotlight indexing state: %w: %s", err, strings.TrimSpace(string(out)))
		}
	}
	volumes, err := parseSpotlightVolumes(string(out))
	if err != nil {
		return err
	}
	backups, err := spotlightBackupMounts(ctx)
	if err != nil {
		return err
	}
	var disabled bool
	for _, volume := range volumes {
		if backups[volume.path] {
			info.SpotlightExcluded = append(info.SpotlightExcluded, volume.path)
			continue
		}
		// "No index" on e.g. an SMB mount does not describe an indexing setting.
		if !volume.hasIndex {
			continue
		}
		info.SpotlightVolumes = append(info.SpotlightVolumes, volume.path)
		if volume.enabled {
			info.SpotlightIndexing = true
		} else {
			disabled = true
		}
	}
	if len(info.SpotlightVolumes) == 0 && len(info.SpotlightExcluded) == 0 {
		return fmt.Errorf("query Spotlight: no indexing status returned")
	}
	info.SpotlightIndexingMixed = info.SpotlightIndexing && disabled
	return nil
}

// spotlightBackupMounts identifies APFS backup volumes by their filesystem
// role, never their display name. Apple requires indexing on these volumes.
func spotlightBackupMounts(ctx context.Context) (map[string]bool, error) {
	out, err := exec.CommandContext(ctx, "diskutil", "apfs", "list", "-plist").Output()
	if err != nil {
		return nil, fmt.Errorf("identify Spotlight backup volumes: %w", err)
	}
	var apfs struct {
		Containers []struct {
			Volumes []struct {
				DeviceIdentifier string
				Roles            []string
			}
		}
	}
	if _, err := plist.Unmarshal(out, &apfs); err != nil {
		return nil, fmt.Errorf("decode APFS volume metadata: %w", err)
	}
	if apfs.Containers == nil {
		return nil, fmt.Errorf("missing Containers in APFS volume metadata")
	}
	backups := make(map[string]bool)
	for _, container := range apfs.Containers {
		if container.Volumes == nil {
			return nil, fmt.Errorf("missing Volumes in APFS container metadata")
		}
		for _, volume := range container.Volumes {
			if volume.Roles == nil {
				return nil, fmt.Errorf("missing Roles in APFS volume metadata")
			}
			if !slices.Contains(volume.Roles, "Backup") {
				continue
			}
			device := volume.DeviceIdentifier
			if !strings.HasPrefix(device, "disk") || strings.ContainsAny(device, "/ \t\r\n") {
				return nil, fmt.Errorf("invalid APFS backup device identifier %q", device)
			}
			out, err := exec.CommandContext(ctx, "diskutil", "info", "-plist", device).Output()
			if err != nil {
				return nil, fmt.Errorf("query backup mount point for %s: %w", device, err)
			}
			var details struct {
				DeviceIdentifier string
				MountPoint       *string
			}
			if _, err := plist.Unmarshal(out, &details); err != nil {
				return nil, fmt.Errorf("decode backup mount metadata for %s: %w", device, err)
			}
			if details.DeviceIdentifier != device {
				return nil, fmt.Errorf("backup mount metadata does not identify %s", device)
			}
			// diskutil represents unmounted APFS volumes with an explicit
			// empty MountPoint string; there is no Mounted key on current macOS.
			if details.MountPoint != nil && *details.MountPoint == "" {
				continue
			}
			if details.MountPoint == nil || !filepath.IsAbs(*details.MountPoint) {
				return nil, fmt.Errorf("backup mount metadata for %s has no absolute mount point", device)
			}
			backups[*details.MountPoint] = true
		}
	}
	return backups, nil
}

func parseSpotlightVolumes(out string) ([]spotlightVolume, error) {
	var volumes []spotlightVolume
	var path string
	seen := make(map[string]bool)
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasSuffix(line, ":") && strings.HasPrefix(line, "/") {
			if path != "" {
				return nil, fmt.Errorf("query Spotlight: volume %q missing indexing status", path)
			}
			path = strings.TrimSuffix(line, ":")
			continue
		}
		var enabled, hasIndex bool
		switch line {
		case "Indexing enabled.":
			enabled, hasIndex = true, true
		case "Indexing disabled.", "Indexing and searching disabled.":
			hasIndex = true
		case "Spotlight server is disabled.":
			hasIndex = true
			if path == "" {
				path = "/"
			}
		case "No index.":
		default:
			return nil, fmt.Errorf("unrecognized Spotlight indexing status: %q", line)
		}
		if path == "" {
			return nil, fmt.Errorf("query Spotlight: indexing status has no volume")
		}
		if seen[path] {
			return nil, fmt.Errorf("query Spotlight: duplicate volume %q", path)
		}
		seen[path] = true
		volumes = append(volumes, spotlightVolume{path: path, enabled: enabled, hasIndex: hasIndex})
		path = ""
	}
	if path != "" || len(volumes) == 0 {
		return nil, fmt.Errorf("query Spotlight: no indexing status returned")
	}
	return volumes, nil
}

func parseSpotlightIndexingState(out string) (enabled, mixed bool, err error) {
	volumes, err := parseSpotlightVolumes(out)
	if err != nil {
		return false, false, err
	}
	var disabled bool
	for _, volume := range volumes {
		if !volume.hasIndex {
			continue
		}
		if volume.enabled {
			enabled = true
		} else {
			disabled = true
		}
	}
	if !enabled && !disabled {
		return false, false, fmt.Errorf("query Spotlight: no indexing status returned")
	}
	return enabled, enabled && disabled, nil
}

// LaunchdUserDomain returns the launchd domain target for the current user's GUI session.
func LaunchdUserDomain() string { return fmt.Sprintf("gui/%d", os.Getuid()) }

// RemoteControlDaemonService is the launchd service that launches Music.app on the play key.
const RemoteControlDaemonService = "com.apple.rcd"

func remoteControlDaemonEnabled(ctx context.Context) (bool, error) {
	out, err := exec.CommandContext(ctx, "launchctl", "print-disabled", LaunchdUserDomain()).CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("query launchd disabled services: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if !strings.Contains(string(out), "disabled services = {") || !strings.Contains(string(out), "}") {
		return false, fmt.Errorf("unrecognized launchd disabled services output: %q", strings.TrimSpace(string(out)))
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		key, value, found := strings.Cut(strings.TrimSpace(line), "=>")
		if !found || strings.TrimSpace(key) != fmt.Sprintf("%q", RemoteControlDaemonService) {
			continue
		}
		switch strings.TrimSpace(value) {
		case "disabled":
			return false, nil
		case "enabled":
			return true, nil
		default:
			return false, fmt.Errorf("unrecognized launchd service state: %q", line)
		}
	}
	return true, nil
}

// LaunchdServiceLoaded distinguishes an absent service from a failed query.
func LaunchdServiceLoaded(ctx context.Context, domain, service string) (bool, error) {
	out, err := exec.CommandContext(ctx, "launchctl", "print", domain+"/"+service).CombinedOutput()
	if err == nil {
		return true, nil
	}
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	if _, ok := errors.AsType[*exec.ExitError](err); ok && strings.Contains(string(out), fmt.Sprintf("Could not find service %q in domain", service)) {
		return false, nil
	}
	return false, fmt.Errorf("query launchd service %s: %w: %s", service, err, strings.TrimSpace(string(out)))
}

// A missing key uses the macOS default, which is enabled. Other errors must
// remain visible, particularly cancellation and failures to start defaults.
func clickWallpaperToShowDesktopEnabled(ctx context.Context) (bool, error) {
	out, err := exec.CommandContext(ctx, "defaults", "read", "com.apple.WindowManager", "EnableStandardClickToShowDesktop").CombinedOutput()
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	if err != nil {
		if _, ok := errors.AsType[*exec.ExitError](err); ok && strings.Contains(string(out), "The domain/default pair of (com.apple.WindowManager, EnableStandardClickToShowDesktop) does not exist") {
			return true, nil
		}
		return false, fmt.Errorf("query wallpaper click setting: %w: %s", err, strings.TrimSpace(string(out)))
	}
	switch strings.TrimSpace(string(out)) {
	case "0":
		return false, nil
	case "1":
		return true, nil
	default:
		return false, fmt.Errorf("unrecognized wallpaper click setting: %q", strings.TrimSpace(string(out)))
	}
}
