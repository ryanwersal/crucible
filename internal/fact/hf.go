package fact

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
)

// HFInfo holds the state relevant to a single HuggingFace download declaration:
// whether the hf CLI is available, whether the destination already holds a
// completed download, and whether HuggingFace auth is configured.
type HFInfo struct {
	Available     bool // is `hf` on PATH?
	Present       bool // does Dest exist with real downloaded content?
	Authenticated bool // is an HF token configured (env or stored login)?
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

// Collect reports hf availability, destination presence, and auth status.
func (c HFCollector) Collect(_ context.Context) (*HFInfo, error) {
	_, lookErr := exec.LookPath("hf")
	info := &HFInfo{Available: lookErr == nil, Authenticated: hfAuthenticated()}

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

// hfAuthenticated reports whether a HuggingFace token is configured the way the
// hf CLI itself resolves it: the HF_TOKEN environment variable, or a non-empty
// stored-login token file. crucible never reads the token's value — only whether
// one exists — so no secret is handled here.
func hfAuthenticated() bool {
	// HF_TOKEN is the current variable; HUGGING_FACE_HUB_TOKEN is the legacy
	// name huggingface_hub still honors.
	if os.Getenv("HF_TOKEN") != "" || os.Getenv("HUGGING_FACE_HUB_TOKEN") != "" {
		return true
	}
	path := hfTokenPath()
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Size() > 0
}

// hfTokenPath returns the stored-login token file location, honoring the same
// overrides as huggingface_hub: HF_TOKEN_PATH, then $HF_HOME/token, then the
// default ~/.cache/huggingface/token.
func hfTokenPath() string {
	if p := os.Getenv("HF_TOKEN_PATH"); p != "" {
		return p
	}
	if home := os.Getenv("HF_HOME"); home != "" {
		return filepath.Join(home, "token")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".cache", "huggingface", "token")
	}
	return ""
}
