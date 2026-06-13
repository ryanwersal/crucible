package fact

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// OllamaInfo holds the set of locally installed Ollama models.
type OllamaInfo struct {
	Available bool            // is `ollama` on PATH?
	Models    map[string]bool // canonical model reference (e.g. "llama3.1:8b") → present
}

// Has reports whether the given model reference is installed, comparing in
// canonical form so equivalent spellings (missing tag, explicit default
// registry/namespace, mixed case) all match.
func (o *OllamaInfo) Has(ref string) bool {
	if o == nil {
		return false
	}
	return o.Models[CanonicalOllamaRef(ref)]
}

// OllamaCollector lists locally installed Ollama models by reading the on-disk
// manifest store. Reading manifests rather than shelling out to `ollama list`
// keeps the fact accurate even when the Ollama server is not running — querying
// a down server reports zero models, which would wrongly trigger re-pulls of
// very large model weights.
type OllamaCollector struct {
	// ModelsDir overrides the model store location. When empty, $OLLAMA_MODELS
	// is used, falling back to ~/.ollama/models. Tests point this at a fixture.
	ModelsDir string
}

// Collect reports whether ollama is available and which models are installed.
func (c OllamaCollector) Collect(_ context.Context) (*OllamaInfo, error) {
	_, lookErr := exec.LookPath("ollama")
	info := &OllamaInfo{Available: lookErr == nil, Models: make(map[string]bool)}

	dir := c.ModelsDir
	if dir == "" {
		if env := os.Getenv("OLLAMA_MODELS"); env != "" {
			dir = env
		} else if home, err := os.UserHomeDir(); err == nil {
			dir = filepath.Join(home, ".ollama", "models")
		}
	}
	if dir == "" {
		return info, nil
	}

	// Each installed model is a manifest file at
	// manifests/<registry>/<namespace>/<name>/<tag>. A missing manifests dir
	// (ollama never used) simply yields no models.
	manifests := filepath.Join(dir, "manifests")
	_ = filepath.WalkDir(manifests, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(manifests, path)
		if relErr != nil {
			return nil
		}
		if ref := refFromManifestPath(rel); ref != "" {
			info.Models[ref] = true
		}
		return nil
	})
	return info, nil
}

// refFromManifestPath reconstructs a canonical model reference from a manifest
// path relative to the manifests/ directory.
func refFromManifestPath(rel string) string {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) != 4 {
		return ""
	}
	registry, namespace, name, tag := parts[0], parts[1], parts[2], parts[3]
	return CanonicalOllamaRef(registry + "/" + namespace + "/" + name + ":" + tag)
}

// CanonicalOllamaRef normalizes a model reference to the form Ollama uses on
// disk and in `ollama list`: lowercased, with the default registry
// ("registry.ollama.ai") and namespace ("library") stripped, and an absent tag
// defaulted to ":latest". Used to compare desired refs against installed ones;
// the original spelling is still what gets passed to `ollama pull`.
func CanonicalOllamaRef(ref string) string {
	ref = strings.ToLower(strings.TrimSpace(ref))
	ref = strings.TrimPrefix(ref, "registry.ollama.ai/")
	ref = strings.TrimPrefix(ref, "library/")

	// Default the tag to :latest when the final path segment carries no tag.
	// Checking only the final segment avoids mistaking a host:port (e.g.
	// "localhost:11434/foo") for a tag.
	lastSeg := ref
	if i := strings.LastIndex(ref, "/"); i >= 0 {
		lastSeg = ref[i+1:]
	}
	if !strings.Contains(lastSeg, ":") {
		ref += ":latest"
	}
	return ref
}
