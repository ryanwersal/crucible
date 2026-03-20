package script

import (
	"bytes"
	"testing"
)

func TestStripShebang(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []byte
		want  []byte
	}{
		{
			name:  "no shebang",
			input: []byte("const c = require('crucible');\n"),
			want:  []byte("const c = require('crucible');\n"),
		},
		{
			name:  "shebang with code",
			input: []byte("#!/usr/bin/env crucible\nconst c = require('crucible');\n"),
			want:  []byte("                       \nconst c = require('crucible');\n"),
		},
		{
			name:  "shebang no trailing newline",
			input: []byte("#!/usr/bin/env crucible"),
			want:  []byte("                       "),
		},
		{
			name:  "empty input",
			input: []byte{},
			want:  []byte{},
		},
		{
			name:  "nil input",
			input: nil,
			want:  nil,
		},
		{
			name:  "hash but not shebang",
			input: []byte("# comment\ncode\n"),
			want:  []byte("# comment\ncode\n"),
		},
		{
			name:  "shebang preserves line numbers",
			input: []byte("#!/usr/bin/env crucible\nline2\nline3\n"),
			want:  []byte("                       \nline2\nline3\n"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := stripShebang(tt.input)
			if !bytes.Equal(got, tt.want) {
				t.Errorf("stripShebang(%q) =\n  %q\nwant:\n  %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripShebang_DoesNotMutateInput(t *testing.T) {
	t.Parallel()
	input := []byte("#!/usr/bin/env crucible\ncode\n")
	original := make([]byte, len(input))
	copy(original, input)

	_ = stripShebang(input)

	if !bytes.Equal(input, original) {
		t.Error("stripShebang mutated the input slice")
	}
}
