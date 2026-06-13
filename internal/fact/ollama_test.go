package fact

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCanonicalOllamaRef(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"llama3.1:8b":                            "llama3.1:8b",
		"llama3.1":                               "llama3.1:latest",
		"registry.ollama.ai/library/llama3.1:8b": "llama3.1:8b",
		"library/llama3.1:8b":                    "llama3.1:8b",
		"hf.co/DavidAU/Repo:Q4_K_M":              "hf.co/davidau/repo:q4_k_m",
		"hf.co/user/repo":                        "hf.co/user/repo:latest",
		"  Qwen2.5-Coder:7b  ":                   "qwen2.5-coder:7b",
		// A host:port prefix must not be mistaken for a tag.
		"localhost:11434/foo": "localhost:11434/foo:latest",
	}
	for in, want := range cases {
		if got := CanonicalOllamaRef(in); got != want {
			t.Errorf("CanonicalOllamaRef(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestOllamaCollector_ScansManifests(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Lay out manifest files exactly as ollama stores them:
	// manifests/<registry>/<namespace>/<name>/<tag>.
	manifests := []string{
		"registry.ollama.ai/library/llama3.1/8b",
		"registry.ollama.ai/library/qwen2.5-coder/7b",
		"hf.co/davidau/somerepo/q4_k_m",
	}
	for _, m := range manifests {
		p := filepath.Join(dir, "manifests", filepath.FromSlash(m))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	info, err := OllamaCollector{ModelsDir: dir}.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"llama3.1:8b", "qwen2.5-coder:7b", "hf.co/davidau/somerepo:q4_k_m"}
	for _, w := range want {
		if !info.Models[w] {
			t.Errorf("expected model %q in %v", w, info.Models)
		}
	}
	if len(info.Models) != len(want) {
		t.Errorf("got %d models, want %d: %v", len(info.Models), len(want), info.Models)
	}

	// Has() matches equivalent spellings, not just the canonical form.
	for _, spelling := range []string{"llama3.1:8b", "library/llama3.1:8b", "hf.co/DavidAU/SomeRepo:Q4_K_M"} {
		if !info.Has(spelling) {
			t.Errorf("Has(%q) = false, want true", spelling)
		}
	}
	if info.Has("llama3.1:70b") {
		t.Error("Has(llama3.1:70b) = true, want false (different tag)")
	}
}

func TestOllamaCollector_MissingStore(t *testing.T) {
	t.Parallel()
	// A models dir with no manifests subdir yields no models, not an error.
	info, err := OllamaCollector{ModelsDir: t.TempDir()}.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Models) != 0 {
		t.Errorf("expected no models, got %v", info.Models)
	}
}
