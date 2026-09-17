package fact

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"howett.net/plist"
)

func macOSFakeCommand(t *testing.T, name, body string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	if name == "mdutil" {
		body := "#!/bin/sh\nprintf '%s\\n' '<plist version=\"1.0\"><dict><key>Containers</key><array/></dict></plist>'\n"
		if err := os.WriteFile(filepath.Join(dir, "diskutil"), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
}

func TestParseSpotlightIndexingState(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, output            string
		enabled, mixed, wantErr bool
	}{
		{name: "enabled", output: "/:\n\tIndexing enabled.\n", enabled: true},
		{name: "disabled", output: "/:\n\tIndexing disabled.\n"},
		{name: "mixed", output: "/:\n Indexing enabled.\n/Volumes/Data:\n Indexing disabled.\n", enabled: true, mixed: true},
		{name: "search disabled", output: "/:\n Indexing and searching disabled.\n"},
		{name: "server disabled", output: "Spotlight server is disabled.\n"},
		{name: "enabled with unindexed SMB mount", output: "/:\nIndexing enabled.\n/Volumes/.timemachine/server/backup:\nNo index.\n", enabled: true},
		{name: "disabled with unindexed SMB mount", output: "/:\nIndexing disabled.\n/Volumes/.timemachine/server/backup:\nNo index.\n"},
		{name: "only unindexed mounts", output: "/Volumes/SMB:\nNo index.\n", wantErr: true},
		{name: "unindexed with diagnostic error", output: "/:\nIndexing enabled.\n/Volumes/SMB:\nError: unable to read metadata.\nNo index.\n", wantErr: true},
		{name: "empty", wantErr: true},
		{name: "volume only", output: "/:\n", wantErr: true},
		{name: "missing second volume status", output: "/:\nIndexing enabled.\n/Volumes/Data:\n", wantErr: true},
		{name: "unknown", output: "/:\n Unknown indexing state.\n", wantErr: true},
		{name: "partial error", output: "/:\nIndexing enabled.\n/Volumes/Data:\nError: unable to read.\n", wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			enabled, mixed, err := parseSpotlightIndexingState(tt.output)
			if (err != nil) != tt.wantErr || enabled != tt.enabled || mixed != tt.mixed {
				t.Fatalf("got (%v,%v,%v)", enabled, mixed, err)
			}
		})
	}
}

func TestSpotlightEmptyOutputFallback(t *testing.T) {
	for _, tt := range []struct {
		name, fallback string
		wantErr        bool
	}{
		{name: "disabled server", fallback: "echo 'Spotlight server is disabled.'"},
		{name: "still empty", fallback: "exit 0", wantErr: true},
		{name: "query fails", fallback: "exit 1", wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			macOSFakeCommand(t, "mdutil", "case \"$*\" in\n '-a -s') exit 0;;\n '-s /') "+tt.fallback+";;\n *) exit 99;;\nesac\n")
			info, err := (MacOSCollector{SpotlightIndexing: true}).Collect(t.Context())
			if (err != nil) != tt.wantErr {
				t.Fatalf("Collect error = %v", err)
			}
			if err == nil && info.SpotlightIndexing {
				t.Fatal("server disabled must mean indexing disabled")
			}
		})
	}
}

func TestWallpaperCollection(t *testing.T) {
	for _, tt := range []struct {
		name, body       string
		enabled, wantErr bool
	}{
		{name: "off", body: "echo 0"}, {name: "on", body: "echo 1", enabled: true},
		{name: "missing", body: "echo 'The domain/default pair of (com.apple.WindowManager, EnableStandardClickToShowDesktop) does not exist' >&2; exit 1", enabled: true},
		{name: "permission failure", body: "echo 'permission denied' >&2; exit 1", wantErr: true},
		{name: "malformed", body: "echo potato", wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// PATH contains only defaults: collection must not invoke unrelated tools.
			macOSFakeCommand(t, "defaults", tt.body)
			info, err := (MacOSCollector{ClickWallpaperToShowDesktop: true}).Collect(t.Context())
			if (err != nil) != tt.wantErr {
				t.Fatalf("Collect error = %v", err)
			}
			if err == nil && info.ClickWallpaperToShowDesktop != tt.enabled {
				t.Fatalf("enabled = %v", info.ClickWallpaperToShowDesktop)
			}
		})
	}
}

func TestLaunchdServiceLoaded(t *testing.T) {
	for _, tt := range []struct {
		name, body      string
		loaded, wantErr bool
	}{
		{name: "loaded", body: "echo 'service = {}'", loaded: true},
		{name: "absent", body: "echo 'Could not find service \"com.apple.rcd\" in domain for user gui: 502' >&2; exit 113"},
		{name: "permission denied", body: "echo 'Operation not permitted' >&2; exit 1", wantErr: true},
		{name: "missing domain", body: "echo 'Could not find domain' >&2; exit 113", wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			macOSFakeCommand(t, "launchctl", tt.body)
			loaded, err := LaunchdServiceLoaded(t.Context(), "gui/502", RemoteControlDaemonService)
			if loaded != tt.loaded || (err != nil) != tt.wantErr {
				t.Fatalf("got (%v,%v)", loaded, err)
			}
		})
	}
}

func TestMusicCollection(t *testing.T) {
	for _, tt := range []struct {
		name, output     string
		enabled, wantErr bool
	}{
		{name: "default enabled", output: "disabled services = {\n}", enabled: true},
		{name: "explicit enabled", output: "disabled services = {\n\"com.apple.rcd\" => enabled\n}", enabled: true},
		{name: "disabled", output: "disabled services = {\n\"com.apple.rcd\"   =>   disabled\n}"},
		{name: "empty", wantErr: true},
		{name: "unknown value", output: "disabled services = {\n\"com.apple.rcd\" => unknown\n}", wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DISABLED_OUTPUT", tt.output)
			macOSFakeCommand(t, "launchctl", "case \"$1\" in\n print-disabled) printf '%s\\n' \"$DISABLED_OUTPUT\";;\n print) exit 0;;\n *) exit 99;;\nesac")
			info, err := (MacOSCollector{MusicPlayKey: true}).Collect(t.Context())
			if (err != nil) != tt.wantErr {
				t.Fatalf("Collect error = %v", err)
			}
			if err == nil && (info.MusicPlayKey != tt.enabled || !info.MusicPlayKeyLoaded) {
				t.Fatalf("state = %+v", info)
			}
		})
	}
}

func TestMacOSCollectionCancellation(t *testing.T) {
	macOSFakeCommand(t, "defaults", "echo 0")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := (MacOSCollector{ClickWallpaperToShowDesktop: true}).Collect(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
}

func spotlightMetadataFixture(t *testing.T, roles []string, mount string) {
	t.Helper()
	list, err := plist.Marshal(map[string]any{"Containers": []any{map[string]any{"Volumes": []any{map[string]any{"DeviceIdentifier": "disk5s1", "Roles": roles, "Name": "Arbitrary display name"}}}}}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	info, err := plist.Marshal(map[string]any{"DeviceIdentifier": "disk5s1", "MountPoint": mount}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("APFS_LIST", string(list))
	t.Setenv("APFS_INFO", string(info))
	body := `#!/bin/sh
case "$*" in
 'apfs list -plist') printf '%s\n' "$APFS_LIST";;
 'info -plist disk5s1') printf '%s\n' "$APFS_INFO";;
 *) exit 99;;
esac
`
	if err := os.WriteFile(filepath.Join(os.Getenv("PATH"), "diskutil"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestSpotlightBackupVolumeExclusion(t *testing.T) {
	for _, tt := range []struct {
		name, status, mount     string
		roles                   []string
		eligible, excluded      []string
		enabled, mixed, wantErr bool
	}{
		{
			name:   "real Time Machine failure",
			status: "/:\nIndexing disabled.\n/System/Volumes/Data:\nIndexing disabled.\n/System/Volumes/Preboot:\nIndexing disabled.\n/Volumes/.timemachine/server/backup:\nNo index.\n/Volumes/Backups of RWM5 2:\nIndexing enabled.\n",
			mount:  "/Volumes/Backups of RWM5 2", roles: []string{"Backup"},
			eligible: []string{"/", "/System/Volumes/Data", "/System/Volumes/Preboot"}, excluded: []string{"/Volumes/Backups of RWM5 2"},
		},
		{name: "role overrides arbitrary name", status: "/:\nIndexing enabled.\n/Volumes/Ordinary Drive:\nIndexing disabled.\n", mount: "/Volumes/Ordinary Drive", roles: []string{"Backup"}, eligible: []string{"/"}, excluded: []string{"/Volumes/Ordinary Drive"}, enabled: true},
		{name: "backup sounding name is eligible without role", status: "/:\nIndexing disabled.\n/Volumes/Time Machine Backups:\nIndexing enabled.\n", mount: "/Volumes/Time Machine Backups", roles: []string{"Data"}, eligible: []string{"/", "/Volumes/Time Machine Backups"}, enabled: true, mixed: true},
		{name: "all backup volumes", status: "/Volumes/Archive:\nIndexing enabled.\n", mount: "/Volumes/Archive", roles: []string{"Backup"}, excluded: []string{"/Volumes/Archive"}},
		{name: "exact mount only", status: "/Volumes/Archive:\nIndexing enabled.\n/Volumes/Archive Copy:\nIndexing enabled.\n", mount: "/Volumes/Archive", roles: []string{"Backup"}, eligible: []string{"/Volumes/Archive Copy"}, excluded: []string{"/Volumes/Archive"}, enabled: true},
		{name: "unmounted backup ignored", status: "/:\nIndexing disabled.\n", roles: []string{"Backup"}, eligible: []string{"/"}},
		{name: "no indexed mounts remains error", status: "/Volumes/SMB:\nNo index.\n", roles: []string{"Data"}, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("MDUTIL_STATUS", tt.status)
			macOSFakeCommand(t, "mdutil", "[ \"$*\" = '-a -s' ] || exit 99; printf '%s\\n' \"$MDUTIL_STATUS\"")
			spotlightMetadataFixture(t, tt.roles, tt.mount)
			info, err := (MacOSCollector{SpotlightIndexing: true}).Collect(t.Context())
			if (err != nil) != tt.wantErr {
				t.Fatalf("Collect error = %v", err)
			}
			if err != nil {
				return
			}
			if info.SpotlightIndexing != tt.enabled || info.SpotlightIndexingMixed != tt.mixed || !slices.Equal(info.SpotlightVolumes, tt.eligible) || !slices.Equal(info.SpotlightExcluded, tt.excluded) {
				t.Fatalf("state = %+v", info)
			}
		})
	}
}

func TestSpotlightMetadataErrors(t *testing.T) {
	for _, tt := range []struct{ name, command, list, info string }{
		{name: "list command failed", command: "exit 1"},
		{name: "malformed list", list: "garbage"},
		{name: "missing containers", list: "<plist><dict/></plist>"},
		{name: "missing volumes", list: "<plist><dict><key>Containers</key><array><dict/></array></dict></plist>"},
		{name: "info command failed", command: "if [ \"$1\" = info ]; then exit 1; fi; printf '%s\\n' \"$APFS_LIST\""},
		{name: "malformed info", info: "garbage"},
		{name: "missing mount metadata", info: "<plist><dict><key>DeviceIdentifier</key><string>disk5s1</string></dict></plist>"},
		{name: "wrong device", info: "<plist><dict><key>DeviceIdentifier</key><string>disk8s1</string><key>MountPoint</key><string>/Volumes/Archive</string></dict></plist>"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			macOSFakeCommand(t, "mdutil", "printf '/:\\nIndexing disabled.\\n'")
			spotlightMetadataFixture(t, []string{"Backup"}, "/Volumes/Archive")
			if tt.list != "" {
				t.Setenv("APFS_LIST", tt.list)
			}
			if tt.info != "" {
				t.Setenv("APFS_INFO", tt.info)
			}
			if tt.command != "" {
				if err := os.WriteFile(filepath.Join(os.Getenv("PATH"), "diskutil"), []byte("#!/bin/sh\n"+tt.command), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			_, err := (MacOSCollector{SpotlightIndexing: true}).Collect(t.Context())
			if err == nil {
				t.Fatal("metadata errors must not silently include backup volumes")
			}
		})
	}
}
