package resource

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/script/decl"
)

// SymlinkHandler plans actions for symlink declarations.
type SymlinkHandler struct{}

func (SymlinkHandler) DeclType() decl.Type { return decl.Symlink }
func (SymlinkHandler) DeclName() string    { return "Symlink" }

func (SymlinkHandler) Plan(ctx context.Context, store *fact.Store, env Env, d decl.Declaration) (PlanOutput, error) {
	// Resolve relative targets against the script's source directory so the
	// created symlink stores an absolute path. Without this, a target like
	// "dotfiles/zsh/zshrc" would be resolved by the OS relative to the link's
	// own directory (e.g. ~/) and dangle.
	target := d.LinkTarget
	if target != "" && !filepath.IsAbs(target) {
		target = filepath.Join(env.SourceDir, target)
	}

	symlinkFact, err := fact.Get(ctx, store, "symlink:"+d.Path, fact.SymlinkCollector{Path: d.Path})
	if err != nil {
		return PlanOutput{}, err
	}
	acts := action.DiffSymlink(action.DesiredSymlink{
		Path:   d.Path,
		Target: target,
		Absent: d.State == decl.Absent,
	}, symlinkFact)
	var out PlanOutput
	if len(acts) == 0 {
		msg := fmt.Sprintf("%s (up to date)", d.Path)
		if d.State == decl.Absent {
			msg = fmt.Sprintf("%s (already absent)", d.Path)
		}
		out.Observations = append(out.Observations, action.Observation{Description: msg})
	} else {
		out.Actions = acts
	}
	return out, nil
}
