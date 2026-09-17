package resource

import (
	"context"
	"fmt"
	"io"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/script/decl"
)

// MacOSHandler plans actions for macOS system tweak declarations.
type MacOSHandler struct{}

func (MacOSHandler) DeclType() decl.Type { return decl.MacOS }
func (MacOSHandler) DeclName() string    { return "MacOS" }

func (MacOSHandler) Plan(ctx context.Context, store *fact.Store, env Env, d decl.Declaration) (PlanOutput, error) {
	collector := fact.MacOSCollector{
		SpotlightIndexing:           d.MacOSTweaks.SpotlightIndexing != nil,
		MusicPlayKey:                d.MacOSTweaks.MusicPlayKey != nil,
		ClickWallpaperToShowDesktop: d.MacOSTweaks.ClickWallpaperToShowDesktop != nil,
	}
	key := fmt.Sprintf("macos:%t:%t:%t", collector.SpotlightIndexing, collector.MusicPlayKey, collector.ClickWallpaperToShowDesktop)
	info, err := fact.Get(ctx, store, key, collector)
	if err != nil {
		return PlanOutput{}, err
	}

	acts := action.DiffMacOS(d.MacOSTweaks, info)

	var out PlanOutput
	for _, path := range info.SpotlightExcluded {
		out.Observations = append(out.Observations, action.Observation{
			Description: fmt.Sprintf("Spotlight indexing unmanaged on Time Machine backup volume %s (required by macOS)", path),
		})
	}
	if len(acts) == 0 {
		out.Observations = append(out.Observations, action.Observation{
			Description: "macOS tweaks (up to date)",
		})
	} else {
		out.Actions = acts
	}

	return out, nil
}

// SetMacOSTweakExecutor applies one macOS system tweak.
type SetMacOSTweakExecutor struct{}

func (SetMacOSTweakExecutor) ActionType() action.Type { return action.SetMacOSTweak }
func (SetMacOSTweakExecutor) ActionName() string      { return "SetMacOSTweak" }

func (SetMacOSTweakExecutor) Execute(ctx context.Context, a action.Action, stdin io.Reader, stdout, stderr io.Writer) error {
	var err error
	var desired decl.MacOSTweaks
	var collector fact.MacOSCollector
	switch a.MacOSTweak {
	case action.TweakSpotlightIndexing:
		state := "on"
		if !a.MacOSTweakEnabled {
			state = "off"
		}
		desired.SpotlightIndexing = &a.MacOSTweakEnabled
		collector.SpotlightIndexing = true
		before, collectErr := collector.Collect(ctx)
		if collectErr != nil {
			return collectErr
		}
		if len(action.DiffMacOS(desired, before)) == 0 {
			return nil
		}
		if len(before.SpotlightVolumes) == 0 {
			return fmt.Errorf("no manageable Spotlight volumes found")
		}
		// Never use -a here: macOS requires indexing on Time Machine
		// backup volumes. Collection excludes those from this target list.
		args := append([]string{"-i", state}, before.SpotlightVolumes...)
		err = runCmd(ctx, a, stdin, stdout, stderr, "mdutil", args...)
	case action.TweakMusicPlayKey:
		err = setRemoteControlDaemon(ctx, a, stdin, stdout, stderr)
		desired.MusicPlayKey = &a.MacOSTweakEnabled
		collector.MusicPlayKey = true
	case action.TweakClickWallpaperToShowDesktop:
		value := "false"
		if a.MacOSTweakEnabled {
			value = "true"
		}
		err = runCmd(ctx, a, stdin, stdout, stderr, "defaults", "write", "com.apple.WindowManager", "EnableStandardClickToShowDesktop", "-bool", value)
		desired.ClickWallpaperToShowDesktop = &a.MacOSTweakEnabled
		collector.ClickWallpaperToShowDesktop = true
	default:
		return fmt.Errorf("unknown macOS tweak %q", a.MacOSTweak)
	}
	if err != nil {
		return err
	}
	// Some macOS tools report success even when a service or system policy
	// prevents the requested change. Check state before reporting completion.
	info, err := collector.Collect(ctx)
	if err != nil {
		return fmt.Errorf("verify %s: %w", a.MacOSTweak.Label(), err)
	}
	if len(action.DiffMacOS(desired, info)) != 0 {
		return fmt.Errorf("%s did not reach requested state (enabled=%t); macOS may restrict this change", a.MacOSTweak.Label(), a.MacOSTweakEnabled)
	}
	return nil
}

func setRemoteControlDaemon(ctx context.Context, a action.Action, stdin io.Reader, stdout, stderr io.Writer) error {
	domain := fact.LaunchdUserDomain()
	service := domain + "/" + fact.RemoteControlDaemonService

	loaded, err := fact.LaunchdServiceLoaded(ctx, domain, fact.RemoteControlDaemonService)
	if err != nil {
		return err
	}

	if a.MacOSTweakEnabled {
		if err := runCmd(ctx, a, stdin, stdout, stderr, "launchctl", "enable", service); err != nil {
			return err
		}

		if loaded {
			return nil
		}

		return runCmd(ctx, a, stdin, stdout, stderr, "launchctl", "bootstrap", domain, "/System/Library/LaunchAgents/"+fact.RemoteControlDaemonService+".plist")
	}

	// Persist the disabled state before unloading so launchd cannot relaunch
	// the service between bootout and disable. A failed bootout is detected on
	// the next plan because collection also checks whether the service is loaded.
	if err := runCmd(ctx, a, stdin, stdout, stderr, "launchctl", "disable", service); err != nil {
		return err
	}
	if loaded {
		return runCmd(ctx, a, stdin, stdout, stderr, "launchctl", "bootout", service)
	}
	return nil
}
