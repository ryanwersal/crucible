package script

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrNoScript is returned when no crucible.js entry point exists in the source directory.
var ErrNoScript = errors.New("no crucible.js found")

const entryPointName = "crucible.js"

// Loader discovers and reads script entry points from a source directory.
type Loader struct {
	sourceDir string
}

// NewLoader creates a Loader that looks for scripts in sourceDir.
func NewLoader(sourceDir string) *Loader {
	return &Loader{sourceDir: sourceDir}
}

// EntryPoint returns the path and content of the crucible.js entry point.
// Returns ErrNoScript if no entry point exists.
func (l *Loader) EntryPoint() (string, []byte, error) {
	path := filepath.Join(l.sourceDir, entryPointName)

	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil, ErrNoScript
	}
	if err != nil {
		return "", nil, fmt.Errorf("read %s: %w", path, err)
	}

	return path, content, nil
}
