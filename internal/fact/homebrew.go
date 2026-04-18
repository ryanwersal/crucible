package fact

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

// HomebrewInfo holds the observed state of Homebrew packages.
type HomebrewInfo struct {
	Available bool            // is `brew` on PATH?
	Formulae  map[string]bool // installed formula names, plus aliases/oldnames/full_name
	Casks     map[string]bool // installed cask tokens, plus old_tokens/full_token
}

// HomebrewCollector collects installed Homebrew formulae and casks.
type HomebrewCollector struct{}

func (h HomebrewCollector) Collect(ctx context.Context) (*HomebrewInfo, error) {
	brewPath, err := exec.LookPath("brew")
	if err != nil {
		return &HomebrewInfo{Available: false}, nil
	}

	cmd := exec.CommandContext(ctx, brewPath, "info", "--json=v2", "--installed")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("brew info: %w", err)
	}

	info, err := parseHomebrewInfo(out)
	if err != nil {
		return nil, err
	}
	info.Available = true
	return info, nil
}

// parseHomebrewInfo parses the JSON output of `brew info --json=v2 --installed`.
// Each installed formula contributes its canonical name, full_name, aliases, and
// oldnames to the Formulae set so that callers can resolve user-provided names
// (like "kubectl" aliasing "kubernetes-cli") to installed state.
func parseHomebrewInfo(data []byte) (*HomebrewInfo, error) {
	var raw struct {
		Formulae []struct {
			Name     string   `json:"name"`
			FullName string   `json:"full_name"`
			Aliases  []string `json:"aliases"`
			Oldnames []string `json:"oldnames"`
		} `json:"formulae"`
		Casks []struct {
			Token     string   `json:"token"`
			FullToken string   `json:"full_token"`
			OldTokens []string `json:"old_tokens"`
		} `json:"casks"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse brew info: %w", err)
	}

	formulae := make(map[string]bool)
	for _, f := range raw.Formulae {
		addName(formulae, f.Name)
		addName(formulae, f.FullName)
		for _, a := range f.Aliases {
			addName(formulae, a)
		}
		for _, o := range f.Oldnames {
			addName(formulae, o)
		}
	}

	casks := make(map[string]bool)
	for _, c := range raw.Casks {
		addName(casks, c.Token)
		addName(casks, c.FullToken)
		for _, t := range c.OldTokens {
			addName(casks, t)
		}
	}

	return &HomebrewInfo{Formulae: formulae, Casks: casks}, nil
}

func addName(set map[string]bool, name string) {
	if name != "" {
		set[name] = true
	}
}
