package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/script"
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
// paths, and returns the actions needed to reconcile them. If a crucible.js
// entry point exists, script-driven planning is used instead.
func (e *Engine) Plan(ctx context.Context) ([]action.Action, error) {
	store := fact.NewStore()

	loader := script.NewLoader(e.sourceDir)
	_, content, err := loader.EntryPoint()
	if errors.Is(err, script.ErrNoScript) {
		return e.planWalk(ctx, store)
	}
	if err != nil {
		return nil, fmt.Errorf("load script: %w", err)
	}
	return e.planScript(ctx, store, content)
}

// planWalk is the original WalkDir-based planning logic.
func (e *Engine) planWalk(ctx context.Context, store *fact.Store) ([]action.Action, error) {
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
		if filepath.Dir(path) == e.sourceDir && (name == "crucible.yaml" || name == "crucible.js") {
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

// planScript executes a crucible.js script and converts the resulting
// declarations into actions by diffing against current system state.
func (e *Engine) planScript(ctx context.Context, store *fact.Store, scriptContent []byte) ([]action.Action, error) {
	// Pre-collect expensive facts concurrently
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(2)

	g.Go(func() error {
		_, err := fact.Get(gctx, store, "os", fact.OSCollector{})
		return err
	})
	g.Go(func() error {
		_, err := fact.Get(gctx, store, "homebrew", fact.HomebrewCollector{})
		return err
	})

	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("pre-collect facts: %w", err)
	}

	// Create and execute script runtime
	rt := script.NewRuntime(ctx, e.logger, e.sourceDir, e.targetDir, store)
	entryPath := filepath.Join(e.sourceDir, "crucible.js")
	decls, err := rt.Execute(ctx, entryPath, scriptContent)
	if err != nil {
		return nil, err
	}

	// Resolve source files and templates
	if err := rt.ResolveContent(ctx, store); err != nil {
		return nil, err
	}
	decls = rt.Declarations()

	// Convert declarations to actions by diffing against system state
	return e.declarationsToActions(ctx, store, decls)
}

// declarationsToActions converts script declarations into actions by
// diffing each declaration against current system state.
func (e *Engine) declarationsToActions(ctx context.Context, store *fact.Store, decls []script.Declaration) ([]action.Action, error) {
	var actions []action.Action
	var packages []action.DesiredPackage

	for _, decl := range decls {
		switch decl.Type {
		case script.DeclFile:
			fileFact, err := fact.Get(ctx, store, "file:"+decl.Path, fact.FileCollector{Path: decl.Path})
			if err != nil {
				return nil, err
			}
			fileActions, err := action.DiffFile(action.DesiredFile{
				Path:    decl.Path,
				Content: decl.Content,
				Mode:    decl.Mode,
			}, fileFact)
			if err != nil {
				return nil, err
			}
			actions = append(actions, fileActions...)

		case script.DeclDir:
			dirFact, err := fact.Get(ctx, store, "dir:"+decl.Path, fact.DirCollector{Path: decl.Path})
			if err != nil {
				return nil, err
			}
			dirActions := action.DiffDir(action.DesiredDir{
				Path: decl.Path,
				Mode: decl.Mode,
			}, dirFact)
			actions = append(actions, dirActions...)

		case script.DeclSymlink:
			symlinkFact, err := fact.Get(ctx, store, "symlink:"+decl.Path, fact.SymlinkCollector{Path: decl.Path})
			if err != nil {
				return nil, err
			}
			symlinkActions := action.DiffSymlink(action.DesiredSymlink{
				Path:   decl.Path,
				Target: decl.LinkTarget,
			}, symlinkFact)
			actions = append(actions, symlinkActions...)

		case script.DeclPackage:
			packages = append(packages, action.DesiredPackage{
				Name: decl.PackageName,
				Type: decl.PackageType,
			})
		}
	}

	// Batch all package declarations into a single DiffHomebrew call
	if len(packages) > 0 {
		brewFact, err := fact.Get(ctx, store, "homebrew", fact.HomebrewCollector{})
		if err != nil {
			return nil, err
		}
		pkgActions, err := action.DiffHomebrew(packages, brewFact)
		if err != nil {
			return nil, err
		}
		actions = append(actions, pkgActions...)
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
