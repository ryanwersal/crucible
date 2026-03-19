package fact

import "testing"

func TestParseMasList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  map[int64]string
	}{
		{
			name:  "typical output",
			input: "497799835  Xcode (15.0)\n409183694  Keynote (14.0)\n",
			want: map[int64]string{
				497799835: "Xcode",
				409183694: "Keynote",
			},
		},
		{
			name:  "empty output",
			input: "",
			want:  map[int64]string{},
		},
		{
			name:  "blank lines ignored",
			input: "\n  \n497799835  Xcode (15.0)\n\n",
			want: map[int64]string{
				497799835: "Xcode",
			},
		},
		{
			name:  "malformed lines skipped",
			input: "not-a-number Foo (1.0)\n497799835  Xcode (15.0)\njunk\n",
			want: map[int64]string{
				497799835: "Xcode",
			},
		},
		{
			name:  "app name with spaces",
			input: "409203825  Final Cut Pro (10.7)\n",
			want: map[int64]string{
				409203825: "Final Cut Pro",
			},
		},
		{
			name:  "no version suffix",
			input: "497799835  Xcode\n",
			want: map[int64]string{
				497799835: "Xcode",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseMasList([]byte(tt.input))
			if len(got) != len(tt.want) {
				t.Fatalf("got %d apps, want %d", len(got), len(tt.want))
			}
			for id, wantName := range tt.want {
				gotName, ok := got[id]
				if !ok {
					t.Errorf("missing app ID %d", id)
					continue
				}
				if gotName != wantName {
					t.Errorf("app %d: got %q, want %q", id, gotName, wantName)
				}
			}
		})
	}
}
