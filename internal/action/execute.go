package action

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ryanwersal/crucible/internal/action/dock"
)

// Execute performs a single action. stdout and stderr from subprocesses are
// written to the provided writers, keeping execution decoupled from the terminal.
func Execute(ctx context.Context, a Action, stdout, stderr io.Writer) error {
	switch a.Type {
	case WriteFile:
		return executeWriteFile(a)
	case CreateDir:
		return os.MkdirAll(a.Path, a.Mode)
	case CreateSymlink:
		return os.Symlink(a.LinkTarget, a.Path)
	case SetPermissions:
		return os.Chmod(a.Path, a.Mode)
	case DeletePath:
		return os.Remove(a.Path)
	case InstallPackage:
		return executeInstallPackage(ctx, a, stdout, stderr)
	case SetDefaults:
		return executeSetDefaults(ctx, a)
	case SetDock:
		return executeSetDock(ctx, a, stdout, stderr)
	case CloneRepo:
		return executeCloneRepo(ctx, a, stdout, stderr)
	case PullRepo:
		return executePullRepo(ctx, a, stdout, stderr)
	case InstallFont:
		return executeInstallFont(a)
	case InstallMiseTool:
		return executeInstallMiseTool(ctx, a, stdout, stderr)
	case SetShell:
		return executeSetShell(ctx, a)
	default:
		return fmt.Errorf("unknown action type: %v", a.Type)
	}
}

func executeWriteFile(a Action) error {
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

func executeInstallPackage(ctx context.Context, a Action, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, "brew", "install", a.PackageName)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

func executeSetDefaults(ctx context.Context, a Action) error {
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

func executeSetDock(ctx context.Context, a Action, stdout, stderr io.Writer) error {
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

func executeCloneRepo(ctx context.Context, a Action, stdout, stderr io.Writer) error {
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

func executePullRepo(ctx context.Context, a Action, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, "git", "-C", a.Path, "pull")
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

func executeInstallFont(a Action) error {
	// Ensure destination directory exists
	dir := filepath.Dir(a.FontDest)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("ensure font dir: %w", err)
	}

	// Copy font file to destination
	src, err := os.ReadFile(a.FontSource)
	if err != nil {
		return fmt.Errorf("read font source %s: %w", a.FontSource, err)
	}

	if err := os.WriteFile(a.FontDest, src, 0o644); err != nil {
		return fmt.Errorf("write font %s: %w", a.FontDest, err)
	}

	return nil
}

func executeInstallMiseTool(ctx context.Context, a Action, stdout, stderr io.Writer) error {
	spec := a.MiseToolName + "@" + a.MiseToolVersion
	cmd := exec.CommandContext(ctx, "mise", "use", "--global", spec)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

func executeSetShell(ctx context.Context, a Action) error {
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
