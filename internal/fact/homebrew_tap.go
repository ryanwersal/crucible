package fact

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// HomebrewTapInfo describes explicitly checked-out Homebrew repositories.
// Core formulae and casks served by Homebrew's API need no checkout.
type HomebrewTapInfo struct {
	Available bool
	Taps      map[string]bool
}

type HomebrewTapCollector struct{}

func (HomebrewTapCollector) Collect(ctx context.Context) (*HomebrewTapInfo, error) {
	brew, err := exec.LookPath("brew")
	if err != nil {
		return &HomebrewTapInfo{}, nil
	}
	brewMu.Lock()
	defer brewMu.Unlock()
	output, err := runBrew(ctx, brew, "tap")
	if err != nil {
		return nil, fmt.Errorf("brew tap: %w", err)
	}
	taps := make(map[string]bool)
	for _, name := range strings.Fields(string(output)) {
		taps[strings.ToLower(name)] = true
	}
	return &HomebrewTapInfo{Available: true, Taps: taps}, nil
}
