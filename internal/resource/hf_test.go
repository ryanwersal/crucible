package resource

import (
	"context"
	"strings"
	"testing"

	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/script/decl"
)

func TestHFDownloadHandler_Plan(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		d               decl.Declaration
		info            *fact.HFInfo
		wantActions     int
		wantObsContains string
	}{
		{
			name:            "present — observation, no action",
			d:               decl.Declaration{Type: decl.HFDownload, HFRepo: "u/r", HFDest: "/d"},
			info:            &fact.HFInfo{Available: true, Present: true},
			wantActions:     0,
			wantObsContains: "(present)",
		},
		{
			name:        "missing — download action",
			d:           decl.Declaration{Type: decl.HFDownload, HFRepo: "u/r", HFDest: "/d"},
			info:        &fact.HFInfo{Available: true, Present: false},
			wantActions: 1,
		},
		{
			name:        "absent and present — destructive remove",
			d:           decl.Declaration{Type: decl.HFDownload, HFRepo: "u/r", HFDest: "/d", State: decl.Absent},
			info:        &fact.HFInfo{Available: true, Present: true},
			wantActions: 1,
		},
		{
			name:            "absent and missing — observation",
			d:               decl.Declaration{Type: decl.HFDownload, HFRepo: "u/r", HFDest: "/d", State: decl.Absent},
			info:            &fact.HFInfo{Available: true, Present: false},
			wantActions:     0,
			wantObsContains: "already absent",
		},
		{
			name:            "hf unavailable — skipped, never errors",
			d:               decl.Declaration{Type: decl.HFDownload, HFRepo: "u/r", HFDest: "/d"},
			info:            &fact.HFInfo{Available: false},
			wantActions:     0,
			wantObsContains: "hf not available",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store := fact.NewStore()
			_, _ = fact.Get(context.Background(), store, "hf:"+tt.d.HFDest, stubHFCollector{info: tt.info})

			out, err := HFDownloadHandler{}.Plan(context.Background(), store, Env{}, tt.d)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(out.Actions) != tt.wantActions {
				t.Errorf("got %d actions, want %d", len(out.Actions), tt.wantActions)
			}
			if tt.wantObsContains != "" {
				if len(out.Observations) == 0 || !strings.Contains(out.Observations[0].Description, tt.wantObsContains) {
					t.Errorf("observations %v should contain %q", out.Observations, tt.wantObsContains)
				}
			}
		})
	}
}

// stubHFCollector returns pre-configured HFInfo without touching disk or PATH.
type stubHFCollector struct {
	info *fact.HFInfo
}

func (s stubHFCollector) Collect(_ context.Context) (*fact.HFInfo, error) {
	return s.info, nil
}
