package action

import (
	"fmt"

	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/script/decl"
)

// MacOSTweak identifies one macOS system tweak managed by c.macos().
type MacOSTweak string

const (
	TweakSpotlightIndexing           MacOSTweak = "spotlightIndexing"
	TweakMusicPlayKey                MacOSTweak = "musicPlayKey"
	TweakClickWallpaperToShowDesktop MacOSTweak = "clickWallpaperToShowDesktop"
)

// Label returns the human-readable name used in plan output.
func (t MacOSTweak) Label() string {
	switch t {
	case TweakSpotlightIndexing:
		return "Spotlight indexing"
	case TweakMusicPlayKey:
		return "Music launch on play key"
	case TweakClickWallpaperToShowDesktop:
		return "click wallpaper to show desktop"
	default:
		return string(t)
	}
}

// NeedsSudo reports whether applying the tweak requires privilege escalation.
func (t MacOSTweak) NeedsSudo() bool {
	return t == TweakSpotlightIndexing
}

// DiffMacOS compares declared tweaks against the observed system state and
// returns one SetMacOSTweak action per tweak that differs.
func DiffMacOS(desired decl.MacOSTweaks, actual *fact.MacOSInfo) []Action {
	if actual == nil {
		actual = &fact.MacOSInfo{}
	}

	var acts []Action
	add := func(t MacOSTweak, want *bool, have bool, inconsistent bool) {
		if want == nil || (*want == have && !inconsistent) {
			return
		}

		verb := "enable"
		if !*want {
			verb = "disable"
		}

		acts = append(acts, Action{
			Type:              SetMacOSTweak,
			SerialGroup:       "macos:" + string(t),
			MacOSTweak:        t,
			MacOSTweakEnabled: *want,
			NeedsSudo:         t.NeedsSudo(),
			Description:       fmt.Sprintf("%s %s", verb, t.Label()),
		})
	}

	// A host with only excluded backup stores has no indexing setting to
	// manage in either direction. Still report its exclusions in the plan.
	if len(actual.SpotlightVolumes) > 0 || len(actual.SpotlightExcluded) == 0 {
		add(TweakSpotlightIndexing, desired.SpotlightIndexing, actual.SpotlightIndexing, actual.SpotlightIndexingMixed)
	}
	add(TweakMusicPlayKey, desired.MusicPlayKey, actual.MusicPlayKey, actual.MusicPlayKey != actual.MusicPlayKeyLoaded)
	add(TweakClickWallpaperToShowDesktop, desired.ClickWallpaperToShowDesktop, actual.ClickWallpaperToShowDesktop, false)

	return acts
}
