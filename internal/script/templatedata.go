package script

import (
	"context"
	"log/slog"

	"github.com/ryanwersal/crucible/internal/fact"
)

// factsToTemplateData builds a nested map from pre-collected facts in the store.
// Keys that fail to load are silently skipped — templates can use the `default`
// function to handle missing values.
func factsToTemplateData(ctx context.Context, logger *slog.Logger, store *fact.Store) map[string]any {
	data := make(map[string]any)

	// OS facts
	if osInfo, err := fact.Get(ctx, store, "os", fact.OSCollector{}); err == nil {
		data["os"] = map[string]any{
			"name":     osInfo.OS,
			"arch":     osInfo.Arch,
			"hostname": osInfo.Hostname,
		}
	} else {
		logger.Debug("skipping os facts for template data", "err", err)
	}

	// Homebrew facts
	if brewInfo, err := fact.Get(ctx, store, "homebrew", fact.HomebrewCollector{}); err == nil {
		data["homebrew"] = map[string]any{
			"available": brewInfo.Available,
		}
	} else {
		logger.Debug("skipping homebrew facts for template data", "err", err)
	}

	return data
}

// mergeTemplateData builds the final template data map. Auto-injected facts
// form the base; user-supplied data overrides at the top level.
func mergeTemplateData(base, user map[string]any) map[string]any {
	if len(user) == 0 {
		return base
	}
	for k, v := range user {
		base[k] = v
	}
	return base
}
