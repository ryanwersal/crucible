package resource

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/script/decl"
)

func macOSCommands(t *testing.T, commands map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	if _, ok := commands["diskutil"]; !ok {
		commands["diskutil"] = `printf '%s' '<plist version="1.0"><dict><key>Containers</key><array/></dict></plist>'`
	}
	for name, body := range commands {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	log := filepath.Join(dir, "calls")
	t.Setenv("CALLS", log)
	return log
}

func TestMacOSPlanOnlyDeclaredTweaksAndCache(t *testing.T) {
	macOSCommands(t, map[string]string{
		"defaults": "echo 0",
		"mdutil":   "echo '/:'; echo 'Indexing enabled.'",
	})
	store := fact.NewStore()
	for _, tt := range []struct {
		tweaks decl.MacOSTweaks
		count  int
	}{
		{decl.MacOSTweaks{ClickWallpaperToShowDesktop: new(false)}, 0},
		{decl.MacOSTweaks{SpotlightIndexing: new(false)}, 1},
	} {
		out, err := (MacOSHandler{}).Plan(t.Context(), store, Env{}, decl.Declaration{MacOSTweaks: tt.tweaks})
		if err != nil {
			t.Fatal(err)
		}
		if len(out.Actions) != tt.count {
			t.Fatalf("actions = %+v", out.Actions)
		}
	}
}

func TestMusicPlayKeyPartialFailureRetry(t *testing.T) {
	for _, enable := range []bool{true, false} {
		t.Run(map[bool]string{true: "bootstrap", false: "bootout"}[enable], func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("STATE_DIR", dir)
			if !enable {
				if err := os.WriteFile(filepath.Join(dir, "loaded"), nil, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			script := `printf '%s\n' "$*" >> "$CALLS"
case "$1" in
 print-disabled)
  echo 'disabled services = {'
  if [ -e "$STATE_DIR/disabled" ]; then echo '"com.apple.rcd" => disabled'; fi
  echo '}';;
 print)
  if [ -e "$STATE_DIR/loaded" ]; then exit 0; fi
  echo 'Could not find service "com.apple.rcd" in domain for user gui: 502' >&2; exit 113;;
 enable) /bin/rm -f "$STATE_DIR/disabled";;
 disable) : > "$STATE_DIR/disabled";;
 bootstrap|bootout)
  if [ ! -e "$STATE_DIR/failed" ]; then : > "$STATE_DIR/failed"; exit 1; fi
  if [ "$1" = bootstrap ]; then : > "$STATE_DIR/loaded"; else /bin/rm -f "$STATE_DIR/loaded"; fi;;
 *) exit 99;;
esac
`
			log := macOSCommands(t, map[string]string{"launchctl": script})
			a := action.Action{Type: action.SetMacOSTweak, MacOSTweak: action.TweakMusicPlayKey, MacOSTweakEnabled: enable}
			exec := SetMacOSTweakExecutor{}
			if err := exec.Execute(t.Context(), a, nil, io.Discard, io.Discard); err == nil {
				t.Fatal("expected first load/unload to fail")
			}
			d := decl.Declaration{MacOSTweaks: decl.MacOSTweaks{MusicPlayKey: &enable}}
			out, err := (MacOSHandler{}).Plan(t.Context(), fact.NewStore(), Env{}, d)
			if err != nil {
				t.Fatal(err)
			}
			if len(out.Actions) != 1 {
				t.Fatalf("partial change must be retried: %+v", out)
			}
			if err := exec.Execute(t.Context(), out.Actions[0], nil, io.Discard, io.Discard); err != nil {
				t.Fatal(err)
			}
			out, err = (MacOSHandler{}).Plan(t.Context(), fact.NewStore(), Env{}, d)
			if err != nil {
				t.Fatal(err)
			}
			if len(out.Actions) != 0 {
				t.Fatalf("retry did not converge: %+v", out)
			}
			calls, err := os.ReadFile(log)
			if err != nil {
				t.Fatal(err)
			}
			domain := fact.LaunchdUserDomain()
			expected := []string{"print " + domain + "/com.apple.rcd", "print-disabled " + domain}
			if enable {
				expected = append(expected, "enable "+domain+"/com.apple.rcd", "bootstrap "+domain+" /System/Library/LaunchAgents/com.apple.rcd.plist")
			} else {
				expected = append(expected, "disable "+domain+"/com.apple.rcd", "bootout "+domain+"/com.apple.rcd")
			}
			for _, command := range expected {
				if !strings.Contains(string(calls), command+"\n") {
					t.Errorf("missing command %q in %s", command, calls)
				}
			}
			if !enable && strings.Index(string(calls), "disable ") > strings.Index(string(calls), "bootout ") {
				t.Fatal("must disable before bootout")
			}
		})
	}
}

func TestSpotlightExecutionVerifiesState(t *testing.T) {
	for _, tt := range []struct {
		name, status string
		wantErr      bool
	}{
		{"disabled", "Indexing disabled.", false},
		{"disabled with unindexed SMB mount", "Indexing disabled.\n/Volumes/SMB:\nNo index.", false},
		{"silent no-op", "Indexing enabled.", true},
		{"mixed volumes", "Indexing disabled.\n/Volumes/Data:\nIndexing enabled.", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("STATUS", tt.status)
			t.Setenv("APPLIED", filepath.Join(t.TempDir(), "applied"))
			log := macOSCommands(t, map[string]string{
				"sudo":   "printf '%s\\n' \"$*\" >> \"$CALLS\"; : > \"$APPLIED\"",
				"mdutil": `if [ -e "$APPLIED" ]; then printf '/:\n%s\n' "$STATUS"; else printf '/:\nIndexing enabled.\n'; fi`,
			})
			err := (SetMacOSTweakExecutor{}).Execute(t.Context(), action.Action{MacOSTweak: action.TweakSpotlightIndexing, MacOSTweakEnabled: false, NeedsSudo: true}, nil, io.Discard, io.Discard)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Execute error = %v", err)
			}
			calls, err := os.ReadFile(log)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(calls), "mdutil -i off /") {
				t.Fatalf("sudo command = %s", calls)
			}
		})
	}
}

func TestWallpaperExecution(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		t.Run(map[bool]string{true: "enable", false: "disable"}[enabled], func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("VALUE", filepath.Join(dir, "value"))
			macOSCommands(t, map[string]string{"defaults": `case "$1" in
 write)
  [ "$2" = com.apple.WindowManager ] && [ "$3" = EnableStandardClickToShowDesktop ] && [ "$4" = -bool ] || exit 99
  if [ "$5" = true ]; then echo 1 > "$VALUE"; else echo 0 > "$VALUE"; fi;;
 read) /bin/cat "$VALUE";;
 *) exit 99;;
esac`})
			err := (SetMacOSTweakExecutor{}).Execute(t.Context(), action.Action{MacOSTweak: action.TweakClickWallpaperToShowDesktop, MacOSTweakEnabled: enabled}, nil, io.Discard, io.Discard)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSpotlightExcludesTimeMachineFromApplyAndVerification(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPLIED", filepath.Join(dir, "applied"))
	log := macOSCommands(t, map[string]string{
		"diskutil": `case "$*" in
 'apfs list -plist') printf '%s' '<plist version="1.0"><dict><key>Containers</key><array><dict><key>Volumes</key><array><dict><key>DeviceIdentifier</key><string>disk5s1</string><key>Roles</key><array><string>Backup</string></array></dict></array></dict></array></dict></plist>';;
 'info -plist disk5s1') printf '%s' '<plist version="1.0"><dict><key>DeviceIdentifier</key><string>disk5s1</string><key>MountPoint</key><string>/Volumes/Backups of RWM5 2</string></dict></plist>';;
 *) exit 98;;
esac`,
		"mdutil": `state=enabled
[ ! -e "$APPLIED" ] || state=disabled
printf '/:\nIndexing %s.\n/System/Volumes/Data:\nIndexing %s.\n/System/Volumes/Preboot:\nIndexing %s.\n/Volumes/Backups of RWM5 2:\nIndexing enabled.\n' "$state" "$state" "$state"`,
		"sudo": `printf '%s\n' "$@" > "$CALLS"
[ "$1" = '-p' ] && [ "$3" = '--' ] || exit 96
shift 3
[ "$1" = mdutil ] && [ "$2" = '-i' ] && [ "$3" = off ] || exit 97
shift 3
[ "$#" = 3 ] && [ "$1" = '/' ] && [ "$2" = /System/Volumes/Data ] && [ "$3" = /System/Volumes/Preboot ] || exit 99
: > "$APPLIED"`,
	})
	d := decl.Declaration{MacOSTweaks: decl.MacOSTweaks{SpotlightIndexing: new(false)}}
	handler := MacOSHandler{}
	plan, err := handler.Plan(t.Context(), fact.NewStore(), Env{}, d)
	if err != nil || len(plan.Actions) != 1 {
		t.Fatalf("plan=%+v err=%v", plan, err)
	}
	if len(plan.Observations) != 1 || !strings.Contains(plan.Observations[0].Description, "Time Machine") {
		t.Fatalf("missing exclusion: %+v", plan)
	}
	if err := (SetMacOSTweakExecutor{}).Execute(t.Context(), plan.Actions[0], nil, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(calls), "Backups") || strings.Contains(string(calls), "-a") {
		t.Fatalf("targeted excluded volume: %s", calls)
	}
	plan, err = handler.Plan(t.Context(), fact.NewStore(), Env{}, d)
	if err != nil || len(plan.Actions) != 0 {
		t.Fatalf("second plan=%+v err=%v", plan, err)
	}
	if len(plan.Observations) != 2 {
		t.Fatalf("missing exclusion/up-to-date observations: %+v", plan)
	}
}
