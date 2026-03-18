package fact

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// FontInfo holds the set of font files installed in a directory.
type FontInfo struct {
	Installed map[string]bool // font filenames present in the directory
}

// FontCollector scans a directory for installed font files.
type FontCollector struct {
	Dir string // e.g. ~/Library/Fonts
}

// Collect reads the font directory and returns filenames of installed fonts.
func (c FontCollector) Collect(_ context.Context) (*FontInfo, error) {
	info := &FontInfo{Installed: make(map[string]bool)}

	entries, err := os.ReadDir(c.Dir)
	if os.IsNotExist(err) {
		return info, nil
	}
	if err != nil {
		return nil, err
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext == ".ttf" || ext == ".otf" || ext == ".ttc" || ext == ".woff" || ext == ".woff2" {
			info.Installed[e.Name()] = true
		}
	}

	return info, nil
}
