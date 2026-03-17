package fact

import (
	"bufio"
	"bytes"
	"context"
	"os/exec"
	"strings"
)

// HomebrewInfo holds the observed state of Homebrew packages.
type HomebrewInfo struct {
	Available bool            // is `brew` on PATH?
	Formulae  map[string]bool // installed formula names
	Casks     map[string]bool // installed cask names
}

// HomebrewCollector collects installed Homebrew formulae and casks.
type HomebrewCollector struct{}

func (h HomebrewCollector) Collect(ctx context.Context) (*HomebrewInfo, error) {
	brewPath, err := exec.LookPath("brew")
	if err != nil {
		return &HomebrewInfo{Available: false}, nil
	}

	formulae, err := listBrewPackages(ctx, brewPath, "--formula")
	if err != nil {
		return nil, err
	}

	casks, err := listBrewPackages(ctx, brewPath, "--cask")
	if err != nil {
		return nil, err
	}

	return &HomebrewInfo{
		Available: true,
		Formulae:  formulae,
		Casks:     casks,
	}, nil
}

func listBrewPackages(ctx context.Context, brewPath, flag string) (map[string]bool, error) {
	cmd := exec.CommandContext(ctx, brewPath, "list", flag, "-1")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	pkgs := make(map[string]bool)
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		name := strings.TrimSpace(scanner.Text())
		if name != "" {
			pkgs[name] = true
		}
	}
	return pkgs, scanner.Err()
}
