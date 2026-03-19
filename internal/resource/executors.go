package resource

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/action/dock"
)

// WriteFileExecutor writes file content atomically.
type WriteFileExecutor struct{}

func (WriteFileExecutor) ActionType() action.Type { return action.WriteFile }

func (WriteFileExecutor) Execute(_ context.Context, a action.Action, _, _ io.Writer) error {
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

func (CreateDirExecutor) Execute(_ context.Context, a action.Action, _, _ io.Writer) error {
	return os.MkdirAll(a.Path, a.Mode)
}

// CreateSymlinkExecutor creates symbolic links.
type CreateSymlinkExecutor struct{}

func (CreateSymlinkExecutor) ActionType() action.Type { return action.CreateSymlink }

func (CreateSymlinkExecutor) Execute(_ context.Context, a action.Action, _, _ io.Writer) error {
	return os.Symlink(a.LinkTarget, a.Path)
}

// SetPermissionsExecutor changes file permissions.
type SetPermissionsExecutor struct{}

func (SetPermissionsExecutor) ActionType() action.Type { return action.SetPermissions }

func (SetPermissionsExecutor) Execute(_ context.Context, a action.Action, _, _ io.Writer) error {
	return os.Chmod(a.Path, a.Mode)
}

// InstallPackageExecutor installs a Homebrew package.
type InstallPackageExecutor struct{}

func (InstallPackageExecutor) ActionType() action.Type { return action.InstallPackage }

func (InstallPackageExecutor) Execute(ctx context.Context, a action.Action, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, "brew", "install", a.PackageName)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

// UninstallPackageExecutor removes a Homebrew package.
type UninstallPackageExecutor struct{}

func (UninstallPackageExecutor) ActionType() action.Type { return action.UninstallPackage }

func (UninstallPackageExecutor) Execute(ctx context.Context, a action.Action, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, "brew", "uninstall", a.PackageName)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

// SetDefaultsExecutor writes a macOS defaults value.
type SetDefaultsExecutor struct{}

func (SetDefaultsExecutor) ActionType() action.Type { return action.SetDefaults }

func (SetDefaultsExecutor) Execute(ctx context.Context, a action.Action, _, _ io.Writer) error {
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

func (DeleteDefaultsExecutor) Execute(ctx context.Context, a action.Action, _, _ io.Writer) error {
	cmd := exec.CommandContext(ctx, "defaults", "delete", a.DefaultsDomain, a.DefaultsKey)
	return cmd.Run()
}

// SetDockExecutor writes the dock layout.
type SetDockExecutor struct{}

func (SetDockExecutor) ActionType() action.Type { return action.SetDock }

func (SetDockExecutor) Execute(ctx context.Context, a action.Action, _, _ io.Writer) error {
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

func (CloneRepoExecutor) Execute(ctx context.Context, a action.Action, stdout, stderr io.Writer) error {
	args := []string{"clone"}
	if a.GitBranch != "" {
		args = append(args, "--branch", a.GitBranch)
	}
	args = append(args, a.GitURL, a.Path)
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

// PullRepoExecutor pulls updates in a git repository.
type PullRepoExecutor struct{}

func (PullRepoExecutor) ActionType() action.Type { return action.PullRepo }

func (PullRepoExecutor) Execute(ctx context.Context, a action.Action, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, "git", "-C", a.Path, "pull")
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

// InstallFontExecutor copies a font file to its destination.
type InstallFontExecutor struct{}

func (InstallFontExecutor) ActionType() action.Type { return action.InstallFont }

func (InstallFontExecutor) Execute(_ context.Context, a action.Action, _, _ io.Writer) error {
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

func (InstallMiseToolExecutor) Execute(ctx context.Context, a action.Action, stdout, stderr io.Writer) error {
	spec := a.MiseToolName + "@" + a.MiseToolVersion
	cmd := exec.CommandContext(ctx, "mise", "use", "--global", spec)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

// UninstallMiseToolExecutor removes a mise tool.
type UninstallMiseToolExecutor struct{}

func (UninstallMiseToolExecutor) ActionType() action.Type { return action.UninstallMiseTool }

func (UninstallMiseToolExecutor) Execute(ctx context.Context, a action.Action, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, "mise", "uninstall", a.MiseToolName)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

// SetShellExecutor changes the user's login shell.
type SetShellExecutor struct{}

func (SetShellExecutor) ActionType() action.Type { return action.SetShell }

func (SetShellExecutor) Execute(ctx context.Context, a action.Action, _, _ io.Writer) error {
	if a.ShellPath == "" || a.ShellPath[0] != '/' {
		return fmt.Errorf("shell path must be an absolute path, got %q", a.ShellPath)
	}

	args := []string{"-s", a.ShellPath}
	if a.ShellUsername != "" {
		args = append(args, a.ShellUsername)
	}

	cmd := exec.CommandContext(ctx, "chsh", args...)
	return cmd.Run()
}
