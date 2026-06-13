package action

import (
	"testing"

	"github.com/ryanwersal/crucible/internal/fact"
)

func TestDiffHF(t *testing.T) {
	t.Parallel()

	t.Run("present — no action", func(t *testing.T) {
		t.Parallel()
		got := DiffHF(DesiredHFDownload{Repo: "u/r", Dest: "/d"}, &fact.HFInfo{Available: true, Present: true})
		if len(got) != 0 {
			t.Fatalf("expected no actions, got %+v", got)
		}
	})

	t.Run("missing — download with flags", func(t *testing.T) {
		t.Parallel()
		got := DiffHF(DesiredHFDownload{
			Repo: "u/r", Dest: "/d", Include: []string{"*.gguf"}, Revision: "main",
		}, &fact.HFInfo{Available: true, Present: false})
		if len(got) != 1 {
			t.Fatalf("expected 1 action, got %d", len(got))
		}
		a := got[0]
		if a.Type != DownloadHF {
			t.Errorf("type = %v, want DownloadHF", a.Type)
		}
		if a.HFRepo != "u/r" || a.HFDest != "/d" || a.HFRevision != "main" {
			t.Errorf("unexpected fields: %+v", a)
		}
		if len(a.HFInclude) != 1 || a.HFInclude[0] != "*.gguf" {
			t.Errorf("include = %v", a.HFInclude)
		}
		if a.SerialGroup != "hf" {
			t.Errorf("SerialGroup = %q, want hf", a.SerialGroup)
		}
	})

	t.Run("absent and present — destructive remove", func(t *testing.T) {
		t.Parallel()
		got := DiffHF(DesiredHFDownload{Repo: "u/r", Dest: "/d", Absent: true}, &fact.HFInfo{Available: true, Present: true})
		if len(got) != 1 {
			t.Fatalf("expected 1 action, got %d", len(got))
		}
		a := got[0]
		if a.Type != DeletePath || !a.Recursive || !a.Destructive {
			t.Errorf("expected recursive destructive DeletePath, got %+v", a)
		}
	})

	t.Run("absent and missing — no action", func(t *testing.T) {
		t.Parallel()
		got := DiffHF(DesiredHFDownload{Repo: "u/r", Dest: "/d", Absent: true}, &fact.HFInfo{Available: true, Present: false})
		if len(got) != 0 {
			t.Fatalf("expected no actions, got %+v", got)
		}
	})
}
