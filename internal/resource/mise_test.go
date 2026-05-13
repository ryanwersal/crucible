package resource

import (
	"context"
	"strings"
	"testing"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/script/decl"
)

func TestMiseToolHandler_PlanBatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		decls           []decl.Declaration
		miseInfo        *fact.MiseInfo
		resolver        action.MiseVersionResolver
		wantActions     int
		wantObs         int
		wantObsContains string
	}{
		{
			name: "tool at correct version",
			decls: []decl.Declaration{
				{Type: decl.MiseTool, MiseToolName: "python", MiseToolVersion: "3.12"},
			},
			miseInfo:        &fact.MiseInfo{Available: true, Globals: map[string]string{"python": "3.12"}},
			wantActions:     0,
			wantObs:         1,
			wantObsContains: "python@3.12 (installed)",
		},
		{
			name: "tool at wrong version",
			decls: []decl.Declaration{
				{Type: decl.MiseTool, MiseToolName: "python", MiseToolVersion: "3.12"},
			},
			miseInfo:    &fact.MiseInfo{Available: true, Globals: map[string]string{"python": "3.11"}},
			wantActions: 1,
			wantObs:     0,
		},
		{
			name: "absent tool already absent",
			decls: []decl.Declaration{
				{Type: decl.MiseTool, MiseToolName: "python", State: decl.Absent},
			},
			miseInfo:        &fact.MiseInfo{Available: true, Globals: map[string]string{}},
			wantActions:     0,
			wantObs:         1,
			wantObsContains: "already absent",
		},
		{
			name: "latest spec already satisfied — observation, no action",
			decls: []decl.Declaration{
				{Type: decl.MiseTool, MiseToolName: "github-cli", MiseToolVersion: "latest"},
			},
			miseInfo:        &fact.MiseInfo{Available: true, Globals: map[string]string{"github-cli": "2.92.0"}},
			resolver:        stubResolver{resolutions: map[string]string{"github-cli@latest": "2.92.0"}},
			wantActions:     0,
			wantObs:         1,
			wantObsContains: "github-cli@latest (installed)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store := fact.NewStore()
			// Pre-populate the store with mise info so the handler doesn't shell out.
			_, _ = fact.Get(context.Background(), store, "mise", stubMiseCollector{info: tt.miseInfo})

			handler := MiseToolHandler{Resolver: tt.resolver}
			out, err := handler.PlanBatch(context.Background(), store, Env{}, tt.decls)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(out.Actions) != tt.wantActions {
				t.Errorf("got %d actions, want %d", len(out.Actions), tt.wantActions)
			}
			if len(out.Observations) != tt.wantObs {
				t.Errorf("got %d observations, want %d", len(out.Observations), tt.wantObs)
			}
			if tt.wantObsContains != "" && tt.wantObs > 0 {
				if !strings.Contains(out.Observations[0].Description, tt.wantObsContains) {
					t.Errorf("observation %q should contain %q", out.Observations[0].Description, tt.wantObsContains)
				}
			}
		})
	}
}

// stubMiseCollector returns pre-configured MiseInfo without running mise.
type stubMiseCollector struct {
	info *fact.MiseInfo
}

func (s stubMiseCollector) Collect(_ context.Context) (*fact.MiseInfo, error) {
	return s.info, nil
}

// stubResolver returns pre-configured spec resolutions without shelling out.
type stubResolver struct {
	resolutions map[string]string
}

func (s stubResolver) Resolve(_ context.Context, name, spec string) (string, error) {
	if r, ok := s.resolutions[name+"@"+spec]; ok {
		return r, nil
	}
	return spec, nil
}
