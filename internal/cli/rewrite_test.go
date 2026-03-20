package cli

import (
	"slices"
	"testing"
)

func TestRewriteScriptArgs(t *testing.T) {
	t.Parallel()

	subs := []string{"apply", "version", "reference"}

	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "bare script",
			args: []string{"./crucible.js"},
			want: []string{"apply", "--file", "./crucible.js"},
		},
		{
			name: "script with flags before",
			args: []string{"--verbose", "./crucible.js", "--dry-run"},
			want: []string{"--verbose", "apply", "--file", "./crucible.js", "--dry-run"},
		},
		{
			name: "known subcommand unchanged",
			args: []string{"apply", "--dry-run"},
			want: []string{"apply", "--dry-run"},
		},
		{
			name: "version subcommand unchanged",
			args: []string{"version"},
			want: []string{"version"},
		},
		{
			name: "no args",
			args: []string{},
			want: []string{},
		},
		{
			name: "nil args",
			args: nil,
			want: nil,
		},
		{
			name: "help flag only",
			args: []string{"--help"},
			want: []string{"--help"},
		},
		{
			name: "non-js unknown arg",
			args: []string{"unknown"},
			want: []string{"unknown"},
		},
		{
			name: "script with path",
			args: []string{"/tmp/my-setup.js"},
			want: []string{"apply", "--file", "/tmp/my-setup.js"},
		},
		{
			name: "script.js in subdir",
			args: []string{"configs/crucible.js"},
			want: []string{"apply", "--file", "configs/crucible.js"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := RewriteScriptArgs(tt.args, subs)
			if !slices.Equal(got, tt.want) {
				t.Errorf("RewriteScriptArgs(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}
