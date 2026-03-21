package fact

import (
	"testing"
)

func TestParseMiseLsOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{
			name:     "empty output",
			input:    "",
			expected: map[string]string{},
		},
		{
			name:     "single tool",
			input:    "python  3.12.0  ~/.config/mise/config.toml\n",
			expected: map[string]string{"python": "3.12.0"},
		},
		{
			name: "multiple tools",
			input: `node    22.0.0  ~/.config/mise/config.toml
python  3.12.0  ~/.config/mise/config.toml
go      1.23.0  ~/.config/mise/config.toml
`,
			expected: map[string]string{"node": "22.0.0", "python": "3.12.0", "go": "1.23.0"},
		},
		{
			name:     "whitespace lines",
			input:    "\n  \n  python  3.12.0\n  \n",
			expected: map[string]string{"python": "3.12.0"},
		},
		{
			name:     "line with only tool name and no version",
			input:    "python\n",
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseMiseLsOutput([]byte(tt.input))
			if len(got) != len(tt.expected) {
				t.Fatalf("got %d tools, want %d: %v", len(got), len(tt.expected), got)
			}
			for k, wantVer := range tt.expected {
				gotVer, ok := got[k]
				if !ok {
					t.Errorf("missing tool %q", k)
				} else if gotVer != wantVer {
					t.Errorf("tool %q: got version %q, want %q", k, gotVer, wantVer)
				}
			}
		})
	}
}
