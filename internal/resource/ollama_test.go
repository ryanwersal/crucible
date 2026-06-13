package resource

import (
	"context"
	"strings"
	"testing"

	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/script/decl"
)

func TestOllamaModelHandler_PlanBatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		decls           []decl.Declaration
		info            *fact.OllamaInfo
		wantActions     int
		wantObs         int
		wantObsContains string
	}{
		{
			name:            "installed model — observation, no action",
			decls:           []decl.Declaration{{Type: decl.OllamaModel, OllamaModel: "llama3.1:8b"}},
			info:            &fact.OllamaInfo{Available: true, Models: map[string]bool{"llama3.1:8b": true}},
			wantActions:     0,
			wantObs:         1,
			wantObsContains: "llama3.1:8b (installed)",
		},
		{
			name:        "missing model — pull action",
			decls:       []decl.Declaration{{Type: decl.OllamaModel, OllamaModel: "qwen2.5-coder:7b"}},
			info:        &fact.OllamaInfo{Available: true, Models: map[string]bool{}},
			wantActions: 1,
			wantObs:     0,
		},
		{
			name:            "absent and already gone — observation",
			decls:           []decl.Declaration{{Type: decl.OllamaModel, OllamaModel: "ghost:1b", State: decl.Absent}},
			info:            &fact.OllamaInfo{Available: true, Models: map[string]bool{}},
			wantActions:     0,
			wantObs:         1,
			wantObsContains: "already absent",
		},
		{
			name:            "ollama unavailable — skipped, never errors",
			decls:           []decl.Declaration{{Type: decl.OllamaModel, OllamaModel: "llama3.1:8b"}},
			info:            &fact.OllamaInfo{Available: false, Models: map[string]bool{}},
			wantActions:     0,
			wantObs:         1,
			wantObsContains: "ollama not available",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store := fact.NewStore()
			_, _ = fact.Get(context.Background(), store, "ollama", stubOllamaCollector{info: tt.info})

			out, err := OllamaModelHandler{}.PlanBatch(context.Background(), store, Env{}, tt.decls)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(out.Actions) != tt.wantActions {
				t.Errorf("got %d actions, want %d", len(out.Actions), tt.wantActions)
			}
			if len(out.Observations) != tt.wantObs {
				t.Errorf("got %d observations, want %d", len(out.Observations), tt.wantObs)
			}
			if tt.wantObsContains != "" && len(out.Observations) > 0 {
				if !strings.Contains(out.Observations[0].Description, tt.wantObsContains) {
					t.Errorf("observation %q should contain %q", out.Observations[0].Description, tt.wantObsContains)
				}
			}
		})
	}
}

// stubOllamaCollector returns pre-configured OllamaInfo without touching disk.
type stubOllamaCollector struct {
	info *fact.OllamaInfo
}

func (s stubOllamaCollector) Collect(_ context.Context) (*fact.OllamaInfo, error) {
	return s.info, nil
}
