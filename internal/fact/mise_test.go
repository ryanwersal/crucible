package fact

import (
	"testing"
)

func TestParseMiseLsOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected map[string]bool
	}{
		{
			name:     "empty output",
			input:    "",
			expected: map[string]bool{},
		},
		{
			name:     "single tool",
			input:    "python  3.12.0  ~/.config/mise/config.toml\n",
			expected: map[string]bool{"python": true},
		},
		{
			name: "multiple tools",
			input: `node    22.0.0  ~/.config/mise/config.toml
python  3.12.0  ~/.config/mise/config.toml
go      1.23.0  ~/.config/mise/config.toml
`,
			expected: map[string]bool{"node": true, "python": true, "go": true},
		},
		{
			name:     "whitespace lines",
			input:    "\n  \n  python  3.12.0\n  \n",
			expected: map[string]bool{"python": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseMiseLsOutput([]byte(tt.input))
			if len(got) != len(tt.expected) {
				t.Fatalf("got %d tools, want %d: %v", len(got), len(tt.expected), got)
			}
			for k := range tt.expected {
				if !got[k] {
					t.Errorf("missing tool %q", k)
				}
			}
		})
	}
}
