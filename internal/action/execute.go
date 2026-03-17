package action

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Chmod(tmpName, a.Mode); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := os.Rename(tmpName, a.Path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

func executeInstallPackage(ctx context.Context, a Action, stdout, stderr io.Writer) error {
	args := []string{"install"}
	if a.PackageType == "cask" {
		args = append(args, "--cask")
	}
	args = append(args, a.PackageName)

	cmd := exec.CommandContext(ctx, "brew", args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}
