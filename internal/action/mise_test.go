package action

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ryanwersal/crucible/internal/fact"
)

// stubResolver returns a pre-configured resolution per (name, spec) pair.
// Specs missing from the map echo the spec back — sufficient for the tests
// since DiffMise short-circuits on a direct string match before calling the
// resolver, so the echo path is never exercised in practice.
type stubResolver struct {
	resolutions map[string]string // key: "name@spec"
	err         error
}

func (s stubResolver) Resolve(_ context.Context, name, spec string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	if r, ok := s.resolutions[name+"@"+spec]; ok {
		return r, nil
	}
	return spec, nil
}

func TestDiffMise(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		desired         []DesiredMiseTool
		actual          *fact.MiseInfo
		resolver        MiseVersionResolver
		wantActions     int
		wantErr         bool
		wantDescContain string
	}{
		{
			name:    "mise not available",
			desired: []DesiredMiseTool{{Name: "python", Version: "3.12"}},
			actual:  &fact.MiseInfo{Available: false},
			wantErr: true,
		},
		{
			name:    "nil actual",
			desired: []DesiredMiseTool{{Name: "python", Version: "3.12"}},
			actual:  nil,
			wantErr: true,
		},
		{
			name:    "tool at correct version",
			desired: []DesiredMiseTool{{Name: "python", Version: "3.12"}},
			actual: &fact.MiseInfo{
				Available: true,
				Globals:   map[string]string{"python": "3.12"},
			},
			wantActions: 0,
		},
		{
			name:    "tool needs install",
			desired: []DesiredMiseTool{{Name: "python", Version: "3.12"}},
			actual: &fact.MiseInfo{
				Available: true,
				Globals:   map[string]string{},
			},
			wantActions: 1,
		},
		{
			name:    "tool at wrong version",
			desired: []DesiredMiseTool{{Name: "python", Version: "3.12"}},
			actual: &fact.MiseInfo{
				Available: true,
				Globals:   map[string]string{"python": "3.11"},
			},
			wantActions:     1,
			wantDescContain: "3.11 → 3.12",
		},
		{
			name: "mixed installed and missing",
			desired: []DesiredMiseTool{
				{Name: "python", Version: "3.12"},
				{Name: "node", Version: "22"},
			},
			actual: &fact.MiseInfo{
				Available: true,
				Globals:   map[string]string{"python": "3.12"},
			},
			wantActions: 1,
		},
		{
			name:    "absent and installed",
			desired: []DesiredMiseTool{{Name: "python", Absent: true}},
			actual: &fact.MiseInfo{
				Available: true,
				Globals:   map[string]string{"python": "3.12"},
			},
			wantActions: 1,
		},
		{
			name:    "absent and not installed",
			desired: []DesiredMiseTool{{Name: "python", Absent: true}},
			actual: &fact.MiseInfo{
				Available: true,
				Globals:   map[string]string{},
			},
			wantActions: 0,
		},
		{
			name:    "latest already satisfied — no action",
			desired: []DesiredMiseTool{{Name: "github-cli", Version: "latest"}},
			actual: &fact.MiseInfo{
				Available: true,
				Globals:   map[string]string{"github-cli": "2.92.0"},
			},
			resolver:    stubResolver{resolutions: map[string]string{"github-cli@latest": "2.92.0"}},
			wantActions: 0,
		},
		{
			name:    "latest with newer available — upgrade",
			desired: []DesiredMiseTool{{Name: "github-cli", Version: "latest"}},
			actual: &fact.MiseInfo{
				Available: true,
				Globals:   map[string]string{"github-cli": "2.91.0"},
			},
			resolver:        stubResolver{resolutions: map[string]string{"github-cli@latest": "2.92.0"}},
			wantActions:     1,
			wantDescContain: "2.91.0 → 2.92.0",
		},
		{
			name:    "prefix spec satisfied — no action",
			desired: []DesiredMiseTool{{Name: "node", Version: "22"}},
			actual: &fact.MiseInfo{
				Available: true,
				Globals:   map[string]string{"node": "22.22.0"},
			},
			resolver:    stubResolver{resolutions: map[string]string{"node@22": "22.22.0"}},
			wantActions: 0,
		},
		{
			name:    "resolver error — falls back to install",
			desired: []DesiredMiseTool{{Name: "github-cli", Version: "latest"}},
			actual: &fact.MiseInfo{
				Available: true,
				Globals:   map[string]string{"github-cli": "2.91.0"},
			},
			resolver:        stubResolver{err: errStub},
			wantActions:     1,
			wantDescContain: "2.91.0 → latest",
		},
		{
			name:    "nil resolver — falls back to old behavior",
			desired: []DesiredMiseTool{{Name: "github-cli", Version: "latest"}},
			actual: &fact.MiseInfo{
				Available: true,
				Globals:   map[string]string{"github-cli": "2.92.0"},
			},
			resolver:    nil,
			wantActions: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actions, err := DiffMise(context.Background(), tt.desired, tt.actual, tt.resolver)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(actions) != tt.wantActions {
				t.Fatalf("expected %d actions, got %d", tt.wantActions, len(actions))
			}
			for _, a := range actions {
				if a.Type != InstallMiseTool && a.Type != UninstallMiseTool {
					t.Errorf("expected InstallMiseTool or UninstallMiseTool, got %s", a.Type)
				}
			}
			if tt.wantDescContain != "" && tt.wantActions > 0 {
				if !strings.Contains(actions[0].Description, tt.wantDescContain) {
					t.Errorf("description %q should contain %q", actions[0].Description, tt.wantDescContain)
				}
			}
		})
	}
}

// errStub is a sentinel for resolver-error tests.
var errStub = errors.New("resolver failed")
