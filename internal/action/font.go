package action

import (
	"fmt"
	"path/filepath"

	"github.com/ryanwersal/crucible/internal/fact"
)

// DesiredFont describes a font file that should be installed (or removed).
type DesiredFont struct {
	Source  string // path to the font file in the source dir
	Name    string // font filename (e.g. "PragmataPro.ttf")
	DestDir string // destination directory (e.g. ~/Library/Fonts)
	Absent  bool   // true = ensure the font is not installed
}

// DiffFonts compares desired fonts against the currently installed fonts
// and returns actions for any that are missing.
func DiffFonts(desired []DesiredFont, actual *fact.FontInfo) []Action {
	var actions []Action

	installed := make(map[string]bool)
	if actual != nil {
		installed = actual.Installed
	}

	for _, f := range desired {
		isInstalled := installed[f.Name]
		if f.Absent {
			if isInstalled {
				actions = append(actions, Action{
					Type:        DeletePath,
					Path:        filepath.Join(f.DestDir, f.Name),
					Description: fmt.Sprintf("remove font %s", f.Name),
				})
			}
			continue
		}
		if !isInstalled {
			actions = append(actions, Action{
				Type:        InstallFont,
				FontSource:  f.Source,
				FontDest:    filepath.Join(f.DestDir, f.Name),
				Description: fmt.Sprintf("install font %s", f.Name),
			})
		}
	}

	return actions
}
