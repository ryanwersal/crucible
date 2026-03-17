package engine

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/fact"
)

// Engine implements the two-phase plan/apply pipeline.
type Engine struct {
	sourceDir string
	targetDir string
	logger    *slog.Logger
	stdout    io.Writer
	stderr    io.Writer
}

// New creates an Engine that maps sourceDir files onto targetDir.
func New(sourceDir, targetDir string, logger *slog.Logger) *Engine {
	return &Engine{
		sourceDir: sourceDir,
		targetDir: targetDir,
		logger:    logger,
		stdout:    os.Stdout,
		stderr:    os.Stderr,
	}
}

// SetOutput configures the writers used for subprocess output during Apply.
func (e *Engine) SetOutput(stdout, stderr io.Writer) {
	e.stdout = stdout
	e.stderr = stderr
}

// Plan walks the source directory, collects facts about corresponding target
// paths, and returns the actions needed to reconcile them.
func (e *Engine) Plan(ctx context.Context) ([]action.Action, error) {
	store := fact.NewStore()
	var actions []action.Action

	err := filepath.WalkDir(e.sourceDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip dotfiles/dirs and crucible.yaml at the source root
		name := d.Name()
		if strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if name == "crucible.yaml" && filepath.Dir(path) == e.sourceDir {
			return nil
		}

		rel, err := filepath.Rel(e.sourceDir, path)
		if err != nil {
			return fmt.Errorf("compute relative path: %w", err)
		}
		targetPath := filepath.Join(e.targetDir, rel)

		if d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return fmt.Errorf("stat source dir %s: %w", path, err)
			}
			dirFact, err := fact.Get(ctx, store, "dir:"+targetPath, fact.DirCollector{Path: targetPath})
			if err != nil {
				return err
			}
			dirActions := action.DiffDir(action.DesiredDir{
				Path: targetPath,
				Mode: info.Mode().Perm(),
			}, dirFact)
			actions = append(actions, dirActions...)
			return nil
		}

		// Handle symlinks
		if d.Type()&fs.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return fmt.Errorf("readlink %s: %w", path, err)
			}
			symlinkFact, err := fact.Get(ctx, store, "symlink:"+targetPath, fact.SymlinkCollector{Path: targetPath})
			if err != nil {
				return err
			}
			symlinkActions := action.DiffSymlink(action.DesiredSymlink{
				Path:   targetPath,
				Target: linkTarget,
			}, symlinkFact)
			actions = append(actions, symlinkActions...)
			return nil
		}

		// Regular file
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read source file %s: %w", path, err)
		}
		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("stat source file %s: %w", path, err)
		}
		fileFact, err := fact.Get(ctx, store, "file:"+targetPath, fact.FileCollector{Path: targetPath})
		if err != nil {
			return err
		}
		fileActions, err := action.DiffFile(action.DesiredFile{
			Path:    targetPath,
			Content: content,
			Mode:    info.Mode().Perm(),
		}, fileFact)
		if err != nil {
			return err
		}
		actions = append(actions, fileActions...)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk source: %w", err)
	}

	return actions, nil
}

// Apply runs Plan and then executes all resulting actions.
func (e *Engine) Apply(ctx context.Context) error {
	actions, err := e.Plan(ctx)
	if err != nil {
		return err
	}

	for i, a := range actions {
		e.logger.Info("executing", "action", a.Type.String(), "description", a.Description)
		if err := action.Execute(ctx, a, e.stdout, e.stderr); err != nil {
			return fmt.Errorf("action %d/%d failed (%s): %w", i+1, len(actions), a.Description, err)
		}
	}

	return nil
}
