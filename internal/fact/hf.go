package fact

import (
	"context"
	"os"
	"os/exec"
)

// HFInfo holds the state relevant to a single HuggingFace download declaration:
// whether the hf CLI is available, and whether the destination already holds a
// completed download.
type HFInfo struct {
	Available bool // is `hf` on PATH?
	Present   bool // does Dest exist with real downloaded content?
}

// HFCollector reports whether hf is available and whether a download
// destination is already populated.
//
// Presence is intentionally coarse: a destination is "present" once it contains
// at least one entry other than hf's own ".cache" metadata directory. Model
// weights are large and effectively immutable, so this treats a populated
// directory as done rather than re-querying the Hub on every plan. To re-fetch
// (e.g. an updated repo), remove the directory.
type HFCollector struct {
	Dest string // local download directory to inspect
}

// Collect reports hf availability and destination presence.
func (c HFCollector) Collect(_ context.Context) (*HFInfo, error) {
	_, lookErr := exec.LookPath("hf")
	info := &HFInfo{Available: lookErr == nil}

	if c.Dest == "" {
		return info, nil
	}
	entries, err := os.ReadDir(c.Dest)
	if err != nil {
		// Missing or unreadable destination → not present (not an error: the
		// directory simply hasn't been downloaded into yet).
		return info, nil
	}
	for _, e := range entries {
		if e.Name() != ".cache" {
			info.Present = true
			break
		}
	}
	return info, nil
}
