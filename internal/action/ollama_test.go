package action

import (
	"testing"

	"github.com/ryanwersal/crucible/internal/fact"
)

func TestDiffOllama(t *testing.T) {
	t.Parallel()

	actual := &fact.OllamaInfo{
		Available: true,
		Models:    map[string]bool{"llama3.1:8b": true},
	}

	tests := []struct {
		name     string
		desired  []DesiredOllamaModel
		wantType Type
		wantLen  int
	}{
		{
			name:    "present and installed — no action",
			desired: []DesiredOllamaModel{{Ref: "llama3.1:8b"}},
			wantLen: 0,
		},
		{
			name:    "present, equivalent spelling installed — no action",
			desired: []DesiredOllamaModel{{Ref: "library/llama3.1:8b"}},
			wantLen: 0,
		},
		{
			name:     "present and missing — pull",
			desired:  []DesiredOllamaModel{{Ref: "qwen2.5-coder:7b"}},
			wantType: PullOllamaModel,
			wantLen:  1,
		},
		{
			name:     "present, same name different tag — pull",
			desired:  []DesiredOllamaModel{{Ref: "llama3.1:70b"}},
			wantType: PullOllamaModel,
			wantLen:  1,
		},
		{
			name:     "absent and installed — remove",
			desired:  []DesiredOllamaModel{{Ref: "llama3.1:8b", Absent: true}},
			wantType: RemoveOllamaModel,
			wantLen:  1,
		},
		{
			name:    "absent and missing — no action",
			desired: []DesiredOllamaModel{{Ref: "ghost:1b", Absent: true}},
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := DiffOllama(tt.desired, actual)
			if len(got) != tt.wantLen {
				t.Fatalf("got %d actions, want %d: %+v", len(got), tt.wantLen, got)
			}
			if tt.wantLen > 0 {
				if got[0].Type != tt.wantType {
					t.Errorf("got action type %v, want %v", got[0].Type, tt.wantType)
				}
				if got[0].SerialGroup != "ollama" {
					t.Errorf("expected SerialGroup \"ollama\", got %q", got[0].SerialGroup)
				}
				if got[0].OllamaModel != tt.desired[0].Ref {
					t.Errorf("expected original ref %q passed to action, got %q", tt.desired[0].Ref, got[0].OllamaModel)
				}
			}
		})
	}
}
