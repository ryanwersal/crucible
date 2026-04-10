package resource

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/action/dock"
	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/script/decl"
)

// buildCmd creates an exec.Cmd, prepending sudo when a.NeedsSudo is true.
func buildCmd(ctx context.Context, a action.Action, stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) *exec.Cmd {
	if a.NeedsSudo {
		args = append([]string{name}, args...)
		name = "sudo"
	}
	cmd := exec.CommandContext(ctx, name, args...)
	if a.NeedsSudo && stdin != nil {
		cmd.Stdin = stdin
	}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd
}

// WriteFileExecutor writes file content atomically.
type WriteFileExecutor struct{}

func (WriteFileExecutor) ActionType() action.Type { return action.WriteFile }
func (WriteFileExecutor) ActionName() string      { return "WriteFile" }

func (WriteFileExecutor) Execute(_ context.Context, a action.Action, _ io.Reader, _, _ io.Writer) error {
	dir := filepath.Dir(a.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("ensure parent dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".crucible-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(a.Content); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Chmod(tmpName, a.Mode); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := os.Rename(tmpName, a.Path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

// CreateDirExecutor creates directories.
type CreateDirExecutor struct{}

func (CreateDirExecutor) ActionType() action.Type { return action.CreateDir }
func (CreateDirExecutor) ActionName() string      { return "CreateDir" }

func (CreateDirExecutor) Execute(_ context.Context, a action.Action, _ io.Reader, _, _ io.Writer) error {
	return os.MkdirAll(a.Path, a.Mode)
}

// CreateSymlinkExecutor creates symbolic links.
type CreateSymlinkExecutor struct{}

func (CreateSymlinkExecutor) ActionType() action.Type { return action.CreateSymlink }
func (CreateSymlinkExecutor) ActionName() string      { return "CreateSymlink" }

func (CreateSymlinkExecutor) Execute(_ context.Context, a action.Action, _ io.Reader, _, _ io.Writer) error {
	return os.Symlink(a.LinkTarget, a.Path)
}

// SetPermissionsExecutor changes file permissions.
type SetPermissionsExecutor struct{}

func (SetPermissionsExecutor) ActionType() action.Type { return action.SetPermissions }
func (SetPermissionsExecutor) ActionName() string      { return "SetPermissions" }

func (SetPermissionsExecutor) Execute(_ context.Context, a action.Action, _ io.Reader, _, _ io.Writer) error {
	return os.Chmod(a.Path, a.Mode)
}

// InstallPackageExecutor installs a Homebrew package.
type InstallPackageExecutor struct{}

func (InstallPackageExecutor) ActionType() action.Type { return action.InstallPackage }
func (InstallPackageExecutor) ActionName() string      { return "InstallPackage" }

func (InstallPackageExecutor) Execute(ctx context.Context, a action.Action, stdin io.Reader, stdout, stderr io.Writer) error {
	return buildCmd(ctx, a, stdin, stdout, stderr, "brew", "install", a.PackageName).Run()
}

// UninstallPackageExecutor removes a Homebrew package.
type UninstallPackageExecutor struct{}

func (UninstallPackageExecutor) ActionType() action.Type { return action.UninstallPackage }
func (UninstallPackageExecutor) ActionName() string      { return "UninstallPackage" }

func (UninstallPackageExecutor) Execute(ctx context.Context, a action.Action, stdin io.Reader, stdout, stderr io.Writer) error {
	return buildCmd(ctx, a, stdin, stdout, stderr, "brew", "uninstall", a.PackageName).Run()
}

// SetDefaultsExecutor writes a macOS defaults value.
type SetDefaultsExecutor struct{}

func (SetDefaultsExecutor) ActionType() action.Type { return action.SetDefaults }
func (SetDefaultsExecutor) ActionName() string      { return "SetDefaults" }

func (SetDefaultsExecutor) Execute(ctx context.Context, a action.Action, _ io.Reader, _, _ io.Writer) error {
	if a.DefaultsValueType == "" {
		return fmt.Errorf("defaults value type not set for %s %s", a.DefaultsDomain, a.DefaultsKey)
	}

	var valueStr string
	switch a.DefaultsValueType {
	case "bool":
		if v, ok := a.DefaultsValue.(bool); ok && v {
			valueStr = "TRUE"
		} else {
			valueStr = "FALSE"
		}
	case "int":
		valueStr = fmt.Sprintf("%d", a.DefaultsValue)
	case "float":
		valueStr = fmt.Sprintf("%g", a.DefaultsValue)
	default:
		valueStr = fmt.Sprintf("%v", a.DefaultsValue)
	}

	cmd := exec.CommandContext(ctx, "defaults", "write", a.DefaultsDomain, a.DefaultsKey, "-"+a.DefaultsValueType, valueStr)
	return cmd.Run()
}

// DeleteDefaultsExecutor deletes a macOS defaults key.
type DeleteDefaultsExecutor struct{}

func (DeleteDefaultsExecutor) ActionType() action.Type { return action.DeleteDefaults }
func (DeleteDefaultsExecutor) ActionName() string      { return "DeleteDefaults" }

func (DeleteDefaultsExecutor) Execute(ctx context.Context, a action.Action, _ io.Reader, _, _ io.Writer) error {
	cmd := exec.CommandContext(ctx, "defaults", "delete", a.DefaultsDomain, a.DefaultsKey)
	return cmd.Run()
}

// SetDockExecutor writes the dock layout.
type SetDockExecutor struct{}

func (SetDockExecutor) ActionType() action.Type { return action.SetDock }
func (SetDockExecutor) ActionName() string      { return "SetDock" }

func (SetDockExecutor) Execute(ctx context.Context, a action.Action, _ io.Reader, _, _ io.Writer) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}
	plistPath := filepath.Join(homeDir, "Library", "Preferences", "com.apple.dock.plist")

	folders := make([]dock.FolderEntry, len(a.DockFolders))
	for i, f := range a.DockFolders {
		folders[i] = dock.FolderEntry{
			Path:    f.Path,
			View:    f.View,
			Display: f.Display,
		}
	}

	if err := dock.Write(plistPath, a.DockApps, folders); err != nil {
		return err
	}
	return dock.RestartDock(ctx)
}

// CloneRepoExecutor clones a git repository.
type CloneRepoExecutor struct{}

func (CloneRepoExecutor) ActionType() action.Type { return action.CloneRepo }
func (CloneRepoExecutor) ActionName() string      { return "CloneRepo" }

func (CloneRepoExecutor) Execute(ctx context.Context, a action.Action, stdin io.Reader, stdout, stderr io.Writer) error {
	args := []string{"clone"}
	if a.GitBranch != "" {
		args = append(args, "--branch", a.GitBranch)
	}
	args = append(args, a.GitURL, a.Path)
	return buildCmd(ctx, a, stdin, stdout, stderr, "git", args...).Run()
}

// PullRepoExecutor pulls updates in a git repository.
type PullRepoExecutor struct{}

func (PullRepoExecutor) ActionType() action.Type { return action.PullRepo }
func (PullRepoExecutor) ActionName() string      { return "PullRepo" }

func (PullRepoExecutor) Execute(ctx context.Context, a action.Action, stdin io.Reader, stdout, stderr io.Writer) error {
	return buildCmd(ctx, a, stdin, stdout, stderr, "git", "-C", a.Path, "pull").Run()
}

// InstallFontExecutor copies a font file to its destination.
type InstallFontExecutor struct{}

func (InstallFontExecutor) ActionType() action.Type { return action.InstallFont }
func (InstallFontExecutor) ActionName() string      { return "InstallFont" }

func (InstallFontExecutor) Execute(_ context.Context, a action.Action, _ io.Reader, _, _ io.Writer) error {
	dir := filepath.Dir(a.FontDest)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("ensure font dir: %w", err)
	}

	src, err := os.ReadFile(a.FontSource)
	if err != nil {
		return fmt.Errorf("read font source %s: %w", a.FontSource, err)
	}

	if err := os.WriteFile(a.FontDest, src, 0o644); err != nil {
		return fmt.Errorf("write font %s: %w", a.FontDest, err)
	}

	return nil
}

// InstallMiseToolExecutor installs a mise tool globally.
type InstallMiseToolExecutor struct{}

func (InstallMiseToolExecutor) ActionType() action.Type { return action.InstallMiseTool }
func (InstallMiseToolExecutor) ActionName() string      { return "InstallMiseTool" }

func (InstallMiseToolExecutor) Execute(ctx context.Context, a action.Action, stdin io.Reader, stdout, stderr io.Writer) error {
	spec := a.MiseToolName + "@" + a.MiseToolVersion
	return buildCmd(ctx, a, stdin, stdout, stderr, "mise", "use", "--global", spec).Run()
}

// UninstallMiseToolExecutor removes a mise tool.
type UninstallMiseToolExecutor struct{}

func (UninstallMiseToolExecutor) ActionType() action.Type { return action.UninstallMiseTool }
func (UninstallMiseToolExecutor) ActionName() string      { return "UninstallMiseTool" }

func (UninstallMiseToolExecutor) Execute(ctx context.Context, a action.Action, stdin io.Reader, stdout, stderr io.Writer) error {
	return buildCmd(ctx, a, stdin, stdout, stderr, "mise", "uninstall", a.MiseToolName).Run()
}

// SetShellExecutor changes the user's login shell.
type SetShellExecutor struct{}

func (SetShellExecutor) ActionType() action.Type { return action.SetShell }
func (SetShellExecutor) ActionName() string      { return "SetShell" }

func (SetShellExecutor) Execute(ctx context.Context, a action.Action, stdin io.Reader, _, _ io.Writer) error {
	if a.ShellPath == "" || a.ShellPath[0] != '/' {
		return fmt.Errorf("shell path must be an absolute path, got %q", a.ShellPath)
	}

	args := []string{"-s", a.ShellPath}
	if a.ShellUsername != "" {
		args = append(args, a.ShellUsername)
	}

	cmd := exec.CommandContext(ctx, "chsh", args...)
	cmd.Stdin = stdin
	return cmd.Run()
}

// InstallMasAppExecutor installs a Mac App Store app via mas.
type InstallMasAppExecutor struct{}

func (InstallMasAppExecutor) ActionType() action.Type { return action.InstallMasApp }
func (InstallMasAppExecutor) ActionName() string      { return "InstallMasApp" }

func (InstallMasAppExecutor) Execute(ctx context.Context, a action.Action, stdin io.Reader, stdout, stderr io.Writer) error {
	id := fmt.Sprintf("%d", a.MasAppID)
	return buildCmd(ctx, a, stdin, stdout, stderr, "mas", "install", id).Run()
}

// SetKeyRemapExecutor applies keyboard modifier remappings via hidutil
// and writes a LaunchAgent plist for persistence across reboots.
type SetKeyRemapExecutor struct{}

func (SetKeyRemapExecutor) ActionType() action.Type { return action.SetKeyRemap }
func (SetKeyRemapExecutor) ActionName() string      { return "SetKeyRemap" }

func (SetKeyRemapExecutor) Execute(ctx context.Context, a action.Action, _ io.Reader, _, _ io.Writer) error {
	mappingJSON := buildUserKeyMappingJSON(a.KeyRemaps)

	// Apply immediately via hidutil.
	cmd := exec.CommandContext(ctx, "hidutil", "property", "--set", mappingJSON)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("hidutil property --set: %w", err)
	}

	// Write LaunchAgent plist for persistence.
	plist := buildKeyRemapPlist(a.KeyRemaps)
	dir := filepath.Dir(a.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating LaunchAgents directory: %w", err)
	}
	if err := os.WriteFile(a.Path, []byte(plist), 0o644); err != nil {
		return fmt.Errorf("writing LaunchAgent plist: %w", err)
	}

	return nil
}

// RemoveKeyRemapExecutor clears keyboard modifier remappings and removes the LaunchAgent.
type RemoveKeyRemapExecutor struct{}

func (RemoveKeyRemapExecutor) ActionType() action.Type { return action.RemoveKeyRemap }
func (RemoveKeyRemapExecutor) ActionName() string      { return "RemoveKeyRemap" }

func (RemoveKeyRemapExecutor) Execute(ctx context.Context, a action.Action, _ io.Reader, _, _ io.Writer) error {
	// Clear live remappings.
	cmd := exec.CommandContext(ctx, "hidutil", "property", "--set", `{"UserKeyMapping":[]}`)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("hidutil property --set (clear): %w", err)
	}

	// Remove LaunchAgent plist if it exists.
	if err := os.Remove(a.Path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("removing LaunchAgent plist: %w", err)
	}

	return nil
}

// buildUserKeyMappingJSON builds the JSON string for hidutil --set.
func buildUserKeyMappingJSON(remaps []action.KeyRemapEntry) string {
	entries := make([]string, 0, len(remaps))
	for _, r := range remaps {
		src, _ := decl.KeyCode(r.From)
		dst, _ := decl.KeyCode(r.To)
		entries = append(entries, fmt.Sprintf(`{"HIDKeyboardModifierMappingSrc":%d,"HIDKeyboardModifierMappingDst":%d}`, src, dst))
	}
	return fmt.Sprintf(`{"UserKeyMapping":[%s]}`, strings.Join(entries, ","))
}

// buildKeyRemapPlist generates a launchd plist that runs hidutil at login.
func buildKeyRemapPlist(remaps []action.KeyRemapEntry) string {
	mappingJSON := buildUserKeyMappingJSON(remaps)
	escaped := xmlEscape(mappingJSON)
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.crucible.keyremap</string>
	<key>ProgramArguments</key>
	<array>
		<string>/usr/bin/hidutil</string>
		<string>property</string>
		<string>--set</string>
		<string>%s</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>
`, escaped)
}

// SetDisplayExecutor applies display density settings via defaults and CoreGraphics.
type SetDisplayExecutor struct{}

func (SetDisplayExecutor) ActionType() action.Type { return action.SetDisplay }
func (SetDisplayExecutor) ActionName() string      { return "SetDisplay" }

func (SetDisplayExecutor) Execute(ctx context.Context, a action.Action, _ io.Reader, _, stderr io.Writer) error {
	// Set sidebar icon size via NSGlobalDomain defaults.
	if a.DisplaySidebarIconSize != "" {
		val, ok := decl.SidebarIconSizeValue(a.DisplaySidebarIconSize)
		if !ok {
			return fmt.Errorf("unknown sidebar icon size %q", a.DisplaySidebarIconSize)
		}
		cmd := exec.CommandContext(ctx, "defaults", "write", "NSGlobalDomain", "NSTableViewDefaultSizeMode", "-int", fmt.Sprintf("%d", val))
		cmd.Stderr = stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("setting sidebar icon size: %w", err)
		}
	}

	// Set menu bar spacing via currentHost defaults.
	switch a.DisplayMenuBarSpacing {
	case "compact":
		cmd := exec.CommandContext(ctx, "defaults", "-currentHost", "write", "-globalDomain", "NSStatusItemSpacing", "-int", "6")
		cmd.Stderr = stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("setting menu bar spacing: %w", err)
		}
		cmd = exec.CommandContext(ctx, "defaults", "-currentHost", "write", "-globalDomain", "NSStatusItemSelectionPadding", "-int", "4")
		cmd.Stderr = stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("setting menu bar padding: %w", err)
		}
	case "default":
		// Remove custom spacing to restore macOS defaults.
		// Deletion fails if the keys don't exist, which is expected and harmless.
		if err := exec.CommandContext(ctx, "defaults", "-currentHost", "delete", "-globalDomain", "NSStatusItemSpacing").Run(); err != nil {
			slog.Debug("removing NSStatusItemSpacing (may not exist)", "err", err)
		}
		if err := exec.CommandContext(ctx, "defaults", "-currentHost", "delete", "-globalDomain", "NSStatusItemSelectionPadding").Run(); err != nil {
			slog.Debug("removing NSStatusItemSelectionPadding (may not exist)", "err", err)
		}
	}

	// Set resolution via CoreGraphics.
	if a.DisplayResolution != "" {
		if err := fact.SetBuiltInDisplayMode(a.DisplayResolution, a.DisplayHZ); err != nil {
			return fmt.Errorf("setting display resolution: %w", err)
		}
	}

	return nil
}

// RunScriptExecutor runs a shell command to install a tool.
type RunScriptExecutor struct{}

func (RunScriptExecutor) ActionType() action.Type { return action.RunScript }
func (RunScriptExecutor) ActionName() string      { return "RunScript" }

func (RunScriptExecutor) Execute(ctx context.Context, a action.Action, stdin io.Reader, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, "sh", "-c", a.ScriptInstall)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

// xmlEscape escapes special XML characters in a string.
func xmlEscape(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&apos;",
	)
	return r.Replace(s)
}
