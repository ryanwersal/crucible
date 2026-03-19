package resource

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/script/decl"
)

// FontHandler batches all font declarations, grouping by destination directory.
type FontHandler struct{}

func (FontHandler) DeclType() decl.Type { return decl.Font }
func (FontHandler) DeclName() string    { return "Font" }

func (FontHandler) PlanBatch(ctx context.Context, store *fact.Store, env Env, decls []decl.Declaration) (PlanOutput, error) {
	// Build desired fonts from declarations.
	fonts := make([]action.DesiredFont, len(decls))
	for i, d := range decls {
		fonts[i] = action.DesiredFont{
			Source:  filepath.Join(env.SourceDir, d.FontSource),
			Name:    d.FontName,
			DestDir: d.FontDestDir,
			Absent:  d.State == decl.Absent,
		}
	}

	// Group fonts by destination directory for single fact collection per dir.
	byDir := make(map[string][]action.DesiredFont)
	for _, f := range fonts {
		byDir[f.DestDir] = append(byDir[f.DestDir], f)
	}

	var out PlanOutput
	for dir, dirFonts := range byDir {
		fontFact, err := fact.Get(ctx, store, "fonts:"+dir, fact.FontCollector{Dir: dir})
		if err != nil {
			return PlanOutput{}, err
		}
		acts := action.DiffFonts(dirFonts, fontFact)
		isInstalled := fontFact != nil && len(fontFact.Installed) > 0
		for _, df := range dirFonts {
			installed := isInstalled && fontFact.Installed[df.Name]
			if df.Absent && !installed {
				out.Observations = append(out.Observations, action.Observation{
					Description: fmt.Sprintf("font %s (already absent)", df.Name),
				})
			} else if !df.Absent && installed {
				out.Observations = append(out.Observations, action.Observation{
					Description: fmt.Sprintf("font %s (installed)", df.Name),
				})
			}
		}
		out.Actions = append(out.Actions, acts...)
	}
	return out, nil
}
