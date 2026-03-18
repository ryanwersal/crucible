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
// paths, and returns the full result of comparing desired vs actual state.
// If a crucible.js entry point exists, script-driven planning is used instead.
func (e *Engine) Plan(ctx context.Context) (action.PlanResult, error) {
	store := fact.NewStore()

	loader := script.NewLoader(e.sourceDir)
	_, content, err := loader.EntryPoint()
	if errors.Is(err, script.ErrNoScript) {
		return e.planWalk(ctx, store)
	}
	if err != nil {
		return action.PlanResult{}, fmt.Errorf("load script: %w", err)
	}
	return e.planScript(ctx, store, content)
}

// planWalk is the original WalkDir-based planning logic.
func (e *Engine) planWalk(ctx context.Context, store *fact.Store) (action.PlanResult, error) {
	var result action.PlanResult

	err := filepath.WalkDir(e.sourceDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip dotfiles/dirs and crucible.yaml/crucible.js at the source root
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
			acts := action.DiffDir(action.DesiredDir{Path: targetPath, Mode: info.Mode().Perm()}, dirFact)
			if len(acts) == 0 {
				result.Observations = append(result.Observations, action.Observation{
					Description: fmt.Sprintf("%s (up to date)", targetPath),
				})
			} else {
				result.Actions = append(result.Actions, acts...)
			}
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
			acts := action.DiffSymlink(action.DesiredSymlink{Path: targetPath, Target: linkTarget}, symlinkFact)
			if len(acts) == 0 {
				result.Observations = append(result.Observations, action.Observation{
					Description: fmt.Sprintf("%s (up to date)", targetPath),
				})
			} else {
				result.Actions = append(result.Actions, acts...)
			}
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
		acts, err := action.DiffFile(action.DesiredFile{
			Path:    targetPath,
			Content: content,
			Mode:    info.Mode().Perm(),
		}, fileFact)
		if err != nil {
			return err
		}
		if len(acts) == 0 {
			result.Observations = append(result.Observations, action.Observation{
				Description: fmt.Sprintf("%s (up to date)", targetPath),
			})
		} else {
			result.Actions = append(result.Actions, acts...)
		}
		return nil
	})
	if err != nil {
		return action.PlanResult{}, fmt.Errorf("walk source: %w", err)
	}

	return result, nil
}

// planScript executes a crucible.js script and converts the resulting
// declarations into a PlanResult by diffing against current system state.
func (e *Engine) planScript(ctx context.Context, store *fact.Store, scriptContent []byte) (action.PlanResult, error) {
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
		return action.PlanResult{}, fmt.Errorf("pre-collect facts: %w", err)
	}

	// Create and execute script runtime
	rt := script.NewRuntime(ctx, e.logger, e.sourceDir, e.targetDir, store)
	entryPath := filepath.Join(e.sourceDir, "crucible.js")
	_, err := rt.Execute(ctx, entryPath, scriptContent)
	if err != nil {
		return action.PlanResult{}, err
	}

	// Resolve source files and templates
	if err := rt.ResolveContent(ctx, store); err != nil {
		return action.PlanResult{}, err
	}

	return e.declarationsToResult(ctx, store, rt.Declarations())
}

// declarationsToResult converts script declarations into a PlanResult by
// diffing each declaration against current system state.
func (e *Engine) declarationsToResult(ctx context.Context, store *fact.Store, decls []script.Declaration) (action.PlanResult, error) {
	var result action.PlanResult
	var packages []action.DesiredPackage
	var fonts []action.DesiredFont
	var miseTools []action.DesiredMiseTool

	for _, decl := range decls {
		switch decl.Type {
		case script.DeclFile:
			fileFact, err := fact.Get(ctx, store, "file:"+decl.Path, fact.FileCollector{Path: decl.Path})
			if err != nil {
				return action.PlanResult{}, err
			}
			acts, err := action.DiffFile(action.DesiredFile{
				Path:    decl.Path,
				Content: decl.Content,
				Mode:    decl.Mode,
				Absent:  decl.State == script.DeclAbsent,
			}, fileFact)
			if err != nil {
				return action.PlanResult{}, err
			}
			if len(acts) == 0 {
				if decl.State == script.DeclAbsent {
					result.Observations = append(result.Observations, action.Observation{
						Description: fmt.Sprintf("%s (already absent)", decl.Path),
					})
				} else {
					result.Observations = append(result.Observations, action.Observation{
						Description: fmt.Sprintf("%s (up to date)", decl.Path),
					})
				}
			} else {
				result.Actions = append(result.Actions, acts...)
			}

		case script.DeclDir:
			dirFact, err := fact.Get(ctx, store, "dir:"+decl.Path, fact.DirCollector{Path: decl.Path})
			if err != nil {
				return action.PlanResult{}, err
			}
			acts := action.DiffDir(action.DesiredDir{
				Path:   decl.Path,
				Mode:   decl.Mode,
				Absent: decl.State == script.DeclAbsent,
			}, dirFact)
			if len(acts) == 0 {
				if decl.State == script.DeclAbsent {
					result.Observations = append(result.Observations, action.Observation{
						Description: fmt.Sprintf("%s (already absent)", decl.Path),
					})
				} else {
					result.Observations = append(result.Observations, action.Observation{
						Description: fmt.Sprintf("%s (up to date)", decl.Path),
					})
				}
			} else {
				result.Actions = append(result.Actions, acts...)
			}

		case script.DeclSymlink:
			symlinkFact, err := fact.Get(ctx, store, "symlink:"+decl.Path, fact.SymlinkCollector{Path: decl.Path})
			if err != nil {
				return action.PlanResult{}, err
			}
			acts := action.DiffSymlink(action.DesiredSymlink{
				Path:   decl.Path,
				Target: decl.LinkTarget,
				Absent: decl.State == script.DeclAbsent,
			}, symlinkFact)
			if len(acts) == 0 {
				if decl.State == script.DeclAbsent {
					result.Observations = append(result.Observations, action.Observation{
						Description: fmt.Sprintf("%s (already absent)", decl.Path),
					})
				} else {
					result.Observations = append(result.Observations, action.Observation{
						Description: fmt.Sprintf("%s (up to date)", decl.Path),
					})
				}
			} else {
				result.Actions = append(result.Actions, acts...)
			}

		case script.DeclPackage:
			packages = append(packages, action.DesiredPackage{
				Name:   decl.PackageName,
				Absent: decl.State == script.DeclAbsent,
			})

		case script.DeclDefaults:
			factKey := fmt.Sprintf("defaults:%s:%s", decl.DefaultsDomain, decl.DefaultsKey)
			defaultsFact, err := fact.Get(ctx, store, factKey, fact.DefaultsCollector{
				Domain: decl.DefaultsDomain,
				Key:    decl.DefaultsKey,
			})
			if err != nil {
				return action.PlanResult{}, err
			}
			acts := action.DiffDefaults(action.DesiredDefault{
				Domain: decl.DefaultsDomain,
				Key:    decl.DefaultsKey,
				Value:  decl.DefaultsValue,
				Absent: decl.State == script.DeclAbsent,
			}, defaultsFact)
			if len(acts) == 0 {
				if decl.State == script.DeclAbsent {
					result.Observations = append(result.Observations, action.Observation{
						Description: fmt.Sprintf("defaults %s %s (already absent)", decl.DefaultsDomain, decl.DefaultsKey),
					})
				} else {
					result.Observations = append(result.Observations, action.Observation{
						Description: fmt.Sprintf("defaults %s %s (up to date)", decl.DefaultsDomain, decl.DefaultsKey),
					})
				}
			} else {
				result.Actions = append(result.Actions, acts...)
			}

		case script.DeclDock:
			dockFact, err := fact.Get(ctx, store, "dock", fact.DockCollector{HomeDir: e.targetDir})
			if err != nil {
				return action.PlanResult{}, err
			}
			desiredFolders := make([]action.DockFolder, len(decl.DockFolders))
			for i, f := range decl.DockFolders {
				desiredFolders[i] = action.DockFolder{
					Path:    f.Path,
					View:    f.View,
					Display: f.Display,
				}
			}
			acts := action.DiffDock(action.DesiredDock{
				Apps:    decl.DockApps,
				Folders: desiredFolders,
			}, dockFact)
			if len(acts) == 0 {
				result.Observations = append(result.Observations, action.Observation{
					Description: "dock layout (up to date)",
				})
			} else {
				result.Actions = append(result.Actions, acts...)
			}

		case script.DeclGitRepo:
			factKey := fmt.Sprintf("gitrepo:%s", decl.Path)
			repoFact, err := fact.Get(ctx, store, factKey, fact.GitRepoCollector{Path: decl.Path})
			if err != nil {
				return action.PlanResult{}, err
			}
			acts, obs := action.DiffGitRepo(action.DesiredGitRepo{
				Path:   decl.Path,
				URL:    decl.GitURL,
				Branch: decl.GitBranch,
			}, repoFact)
			result.Actions = append(result.Actions, acts...)
			result.Observations = append(result.Observations, obs...)
			if len(acts) == 0 && len(obs) == 0 {
				result.Observations = append(result.Observations, action.Observation{
					Description: fmt.Sprintf("%s (up to date)", decl.Path),
				})
			}

		case script.DeclFont:
			fonts = append(fonts, action.DesiredFont{
				Source:  filepath.Join(e.sourceDir, decl.FontSource),
				Name:    decl.FontName,
				DestDir: decl.FontDestDir,
				Absent:  decl.State == script.DeclAbsent,
			})

		case script.DeclMiseTool:
			miseTools = append(miseTools, action.DesiredMiseTool{
				Name:    decl.MiseToolName,
				Version: decl.MiseToolVersion,
				Absent:  decl.State == script.DeclAbsent,
			})

		case script.DeclShell:
			username := decl.ShellUsername
			shellFact, err := fact.Get(ctx, store, "shell:"+username, fact.ShellCollector{Username: username})
			if err != nil {
				return action.PlanResult{}, err
			}
			acts := action.DiffShell(action.DesiredShell{
				Path:     decl.ShellPath,
				Username: username,
			}, shellFact)
			if len(acts) == 0 {
				result.Observations = append(result.Observations, action.Observation{
					Description: fmt.Sprintf("shell %s (up to date)", decl.ShellPath),
				})
			} else {
				result.Actions = append(result.Actions, acts...)
			}
		}
	}

	// Batch all package declarations into a single DiffHomebrew call
	if len(packages) > 0 {
		brewFact, err := fact.Get(ctx, store, "homebrew", fact.HomebrewCollector{})
		if err != nil {
			return action.PlanResult{}, err
		}
		pkgActions, err := action.DiffHomebrew(packages, brewFact)
		if err != nil {
			return action.PlanResult{}, err
		}

		// Separate packages needing action from those already in desired state
		hasAction := make(map[string]bool, len(pkgActions))
		for _, a := range pkgActions {
			hasAction[a.PackageName] = true
		}
		for _, pkg := range packages {
			if !hasAction[pkg.Name] {
				if pkg.Absent {
					result.Observations = append(result.Observations, action.Observation{
						Description: fmt.Sprintf("%s (already absent)", pkg.Name),
					})
				} else {
					result.Observations = append(result.Observations, action.Observation{
						Description: fmt.Sprintf("%s (installed)", pkg.Name),
					})
				}
			}
		}
		result.Actions = append(result.Actions, pkgActions...)
	}

	// Batch all font declarations
	if len(fonts) > 0 {
		// Group fonts by destination directory for single fact collection per dir
		byDir := make(map[string][]action.DesiredFont)
		for _, f := range fonts {
			byDir[f.DestDir] = append(byDir[f.DestDir], f)
		}
		for dir, dirFonts := range byDir {
			fontFact, err := fact.Get(ctx, store, "fonts:"+dir, fact.FontCollector{Dir: dir})
			if err != nil {
				return action.PlanResult{}, err
			}
			acts := action.DiffFonts(dirFonts, fontFact)
			isInstalled := fontFact != nil && len(fontFact.Installed) > 0
			for _, df := range dirFonts {
				installed := isInstalled && fontFact.Installed[df.Name]
				if df.Absent && !installed {
					result.Observations = append(result.Observations, action.Observation{
						Description: fmt.Sprintf("font %s (already absent)", df.Name),
					})
				} else if !df.Absent && installed {
					result.Observations = append(result.Observations, action.Observation{
						Description: fmt.Sprintf("font %s (installed)", df.Name),
					})
				}
			}
			result.Actions = append(result.Actions, acts...)
		}
	}

	// Batch all mise tool declarations
	if len(miseTools) > 0 {
		miseFact, err := fact.Get(ctx, store, "mise", fact.MiseCollector{})
		if err != nil {
			return action.PlanResult{}, err
		}
		miseActions, err := action.DiffMise(miseTools, miseFact)
		if err != nil {
			return action.PlanResult{}, err
		}

		hasAction := make(map[string]bool, len(miseActions))
		for _, a := range miseActions {
			hasAction[a.MiseToolName] = true
		}
		for _, tool := range miseTools {
			if !hasAction[tool.Name] {
				if tool.Absent {
					result.Observations = append(result.Observations, action.Observation{
						Description: fmt.Sprintf("mise %s (already absent)", tool.Name),
					})
				} else {
					result.Observations = append(result.Observations, action.Observation{
						Description: fmt.Sprintf("mise %s (installed)", tool.Name),
					})
				}
			}
		}
		result.Actions = append(result.Actions, miseActions...)
	}

	return result, nil
}

// Apply runs Plan and then executes all resulting actions, returning the full
// PlanResult so callers can report what was already current and what was applied.
func (e *Engine) Apply(ctx context.Context) (action.PlanResult, error) {
	result, err := e.Plan(ctx)
	if err != nil {
		return action.PlanResult{}, err
	}

	for i, a := range result.Actions {
		e.logger.Info("executing", "action", a.Type.String(), "description", a.Description)
		if err := action.Execute(ctx, a, e.stdout, e.stderr); err != nil {
			return action.PlanResult{}, fmt.Errorf("action %d/%d failed (%s): %w", i+1, len(result.Actions), a.Description, err)
		}
	}

	return result, nil
}
