package fact

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestHFCollector_Presence(t *testing.T) {
	t.Parallel()

	t.Run("populated dest is present", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "model.gguf"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		info, err := HFCollector{Dest: dir}.Collect(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if !info.Present {
			t.Error("expected Present=true for a dest with real content")
		}
	})

	t.Run("only hf .cache metadata is not present", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, ".cache"), 0o755); err != nil {
			t.Fatal(err)
		}
		info, err := HFCollector{Dest: dir}.Collect(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if info.Present {
			t.Error("expected Present=false when only .cache exists")
		}
	})

	t.Run("missing dest is not present and not an error", func(t *testing.T) {
		t.Parallel()
		info, err := HFCollector{Dest: filepath.Join(t.TempDir(), "nope")}.Collect(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if info.Present {
			t.Error("expected Present=false for a missing dest")
		}
	})
}

func TestHFCollector_Authenticated(t *testing.T) {
	// Not parallel: these mutate process env via t.Setenv.

	t.Run("HF_TOKEN env set", func(t *testing.T) {
		t.Setenv("HF_TOKEN", "hf_xxx")
		info, err := HFCollector{}.Collect(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if !info.Authenticated {
			t.Error("expected Authenticated=true when HF_TOKEN is set")
		}
	})

	t.Run("legacy HUGGING_FACE_HUB_TOKEN env set", func(t *testing.T) {
		t.Setenv("HF_TOKEN", "")
		t.Setenv("HUGGING_FACE_HUB_TOKEN", "hf_xxx")
		info, err := HFCollector{}.Collect(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if !info.Authenticated {
			t.Error("expected Authenticated=true when HUGGING_FACE_HUB_TOKEN is set")
		}
	})

	t.Run("stored token file via HF_HOME", func(t *testing.T) {
		t.Setenv("HF_TOKEN", "")
		t.Setenv("HUGGING_FACE_HUB_TOKEN", "")
		t.Setenv("HF_TOKEN_PATH", "")
		home := t.TempDir()
		t.Setenv("HF_HOME", home)
		if err := os.WriteFile(filepath.Join(home, "token"), []byte("hf_xxx"), 0o600); err != nil {
			t.Fatal(err)
		}
		info, err := HFCollector{}.Collect(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if !info.Authenticated {
			t.Error("expected Authenticated=true when a stored token file exists")
		}
	})

	t.Run("no token anywhere", func(t *testing.T) {
		t.Setenv("HF_TOKEN", "")
		t.Setenv("HUGGING_FACE_HUB_TOKEN", "")
		t.Setenv("HF_TOKEN_PATH", "")
		t.Setenv("HF_HOME", t.TempDir()) // empty dir, no token file
		info, err := HFCollector{}.Collect(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if info.Authenticated {
			t.Error("expected Authenticated=false with no token configured")
		}
	})
}
