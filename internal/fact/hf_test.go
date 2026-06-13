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
