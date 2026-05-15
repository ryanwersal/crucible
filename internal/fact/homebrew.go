package fact

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

// HomebrewInfo holds the observed state of Homebrew packages.
type HomebrewInfo struct {
	Available bool                       // is `brew` on PATH?
	Formulae  map[string]bool            // installed formula names, plus aliases/oldnames/full_name
	Casks     map[string]bool            // installed cask tokens, plus old_tokens/full_token
	Outdated  map[string]OutdatedPackage // outdated packages, keyed by every name in Formulae/Casks
}

// OutdatedPackage describes a single package that has a newer version available.
type OutdatedPackage struct {
	Name             string // canonical formula name or cask token
	InstalledVersion string
	CurrentVersion   string
	IsCask           bool
	Pinned           bool // formulae only; pinned formulae are skipped by `brew upgrade`
	AutoUpdates      bool // casks only; cask updates itself outside of brew
}

// HomebrewCollector collects installed Homebrew formulae and casks, plus
// outdated package data. If Refresh is true, the collector runs `brew update`
// before querying state to refresh the tap index. A failed update is logged
// and does not block fact collection (we proceed with the stale index).
type HomebrewCollector struct {
	Refresh bool
}

func (h HomebrewCollector) Collect(ctx context.Context) (*HomebrewInfo, error) {
	brewPath, err := exec.LookPath("brew")
	if err != nil {
		return &HomebrewInfo{Available: false}, nil
	}

	if h.Refresh {
		// brew update is network-dependent; treat failure as a warning so that
		// offline runs still produce a useful (if possibly stale) plan. Capture
		// stderr so the warning explains why brew failed (offline, auth issue, etc).
		cmd := exec.CommandContext(ctx, brewPath, "update", "--quiet")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			slog.WarnContext(ctx, "brew update failed; continuing with stale tap index",
				"err", err, "stderr", strings.TrimSpace(stderr.String()))
		}
	}

	infoCmd := exec.CommandContext(ctx, brewPath, "info", "--json=v2", "--installed")
	infoOut, err := infoCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("brew info: %w", err)
	}

	info, installCtx, err := parseHomebrewInfo(infoOut)
	if err != nil {
		return nil, err
	}
	info.Available = true

	// --greedy includes casks that auto-update themselves (e.g. Chrome). We
	// detect those via the auto_updates flag from `brew info` so they can
	// be surfaced as informational rows instead of attempted upgrades.
	outdatedCmd := exec.CommandContext(ctx, brewPath, "outdated", "--json=v2", "--greedy")
	outdatedOut, err := outdatedCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("brew outdated: %w", err)
	}

	info.Outdated, err = parseHomebrewOutdated(outdatedOut, installCtx)
	if err != nil {
		return nil, err
	}

	return info, nil
}

// installContext carries info derived from `brew info` that the outdated
// parser needs to enrich its entries: alias-resolution data and cask-level
// auto_updates flags (which are not present in `brew outdated` output).
type installContext struct {
	aliases         map[string][]string
	caskAutoUpdates map[string]bool
}

// parseHomebrewInfo parses the JSON output of `brew info --json=v2 --installed`.
// Returns the populated info (Formulae/Casks sets) plus an installContext for
// the outdated parser. Names in the alias map are deduplicated across canonical,
// full, aliases, and oldnames/old_tokens so a single outdated entry can be
// replayed onto every form a user might declare.
func parseHomebrewInfo(data []byte) (*HomebrewInfo, *installContext, error) {
	var raw struct {
		Formulae []struct {
			Name     string   `json:"name"`
			FullName string   `json:"full_name"`
			Aliases  []string `json:"aliases"`
			Oldnames []string `json:"oldnames"`
		} `json:"formulae"`
		Casks []struct {
			Token       string   `json:"token"`
			FullToken   string   `json:"full_token"`
			OldTokens   []string `json:"old_tokens"`
			AutoUpdates bool     `json:"auto_updates"`
		} `json:"casks"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, nil, fmt.Errorf("parse brew info: %w", err)
	}

	formulae := make(map[string]bool)
	casks := make(map[string]bool)
	aliases := make(map[string][]string)
	caskAutoUpdates := make(map[string]bool)

	for _, f := range raw.Formulae {
		names := collectNames(f.Name, f.FullName, f.Aliases, f.Oldnames)
		for _, n := range names {
			formulae[n] = true
		}
		if f.Name != "" {
			aliases[f.Name] = names
		}
	}
	for _, c := range raw.Casks {
		names := collectNames(c.Token, c.FullToken, nil, c.OldTokens)
		for _, n := range names {
			casks[n] = true
		}
		if c.Token != "" {
			aliases[c.Token] = names
			caskAutoUpdates[c.Token] = c.AutoUpdates
		}
	}

	return &HomebrewInfo{Formulae: formulae, Casks: casks},
		&installContext{aliases: aliases, caskAutoUpdates: caskAutoUpdates},
		nil
}

// collectNames returns a de-duplicated list of every name that should resolve
// to a single installed package: canonical, full, plus aliases/oldnames.
func collectNames(canonical, full string, aliasList, oldNames []string) []string {
	seen := make(map[string]bool)
	var out []string
	add := func(s string) {
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}
	add(canonical)
	add(full)
	for _, a := range aliasList {
		add(a)
	}
	for _, o := range oldNames {
		add(o)
	}
	return out
}

// parseHomebrewOutdated parses `brew outdated --json=v2 --greedy`. The result
// is keyed by every alias of each outdated package, so lookups by any name a
// user might declare (canonical, full, alias, oldname) all resolve. The
// installContext supplies the alias mapping and cask auto_updates flags
// (which `brew outdated` does not emit).
func parseHomebrewOutdated(data []byte, ctx *installContext) (map[string]OutdatedPackage, error) {
	var raw struct {
		Formulae []struct {
			Name              string   `json:"name"`
			InstalledVersions []string `json:"installed_versions"`
			CurrentVersion    string   `json:"current_version"`
			Pinned            bool     `json:"pinned"`
		} `json:"formulae"`
		Casks []struct {
			Name              string   `json:"name"`
			InstalledVersions []string `json:"installed_versions"`
			CurrentVersion    string   `json:"current_version"`
		} `json:"casks"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse brew outdated: %w", err)
	}

	out := make(map[string]OutdatedPackage)

	for _, f := range raw.Formulae {
		installed := ""
		if n := len(f.InstalledVersions); n > 0 {
			installed = f.InstalledVersions[n-1]
		}
		pkg := OutdatedPackage{
			Name:             f.Name,
			InstalledVersion: installed,
			CurrentVersion:   f.CurrentVersion,
			Pinned:           f.Pinned,
		}
		recordOutdated(out, f.Name, pkg, ctx.aliases)
	}
	for _, c := range raw.Casks {
		installed := ""
		if n := len(c.InstalledVersions); n > 0 {
			installed = c.InstalledVersions[n-1]
		}
		pkg := OutdatedPackage{
			Name:             c.Name,
			InstalledVersion: installed,
			CurrentVersion:   c.CurrentVersion,
			IsCask:           true,
			AutoUpdates:      ctx.caskAutoUpdates[c.Name],
		}
		recordOutdated(out, c.Name, pkg, ctx.aliases)
	}

	return out, nil
}

func recordOutdated(out map[string]OutdatedPackage, canonical string, pkg OutdatedPackage, aliases map[string][]string) {
	if canonical == "" {
		return
	}
	names, ok := aliases[canonical]
	if !ok {
		names = []string{canonical}
	}
	for _, n := range names {
		out[n] = pkg
	}
}
