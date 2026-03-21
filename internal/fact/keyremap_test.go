package fact

import "testing"

func TestParseHIDUtilOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    []KeyRemapMapping
		wantErr bool
	}{
		{
			name: "single mapping",
			input: `(
        {
            HIDKeyboardModifierMappingDst = 30064771296;
            HIDKeyboardModifierMappingSrc = 30064771129;
        }
    )`,
			want: []KeyRemapMapping{
				{Src: 30064771129, Dst: 30064771296},
			},
		},
		{
			name: "two mappings",
			input: `(
        {
            HIDKeyboardModifierMappingDst = 30064771296;
            HIDKeyboardModifierMappingSrc = 30064771129;
        },
        {
            HIDKeyboardModifierMappingDst = 30064771129;
            HIDKeyboardModifierMappingSrc = 30064771296;
        }
    )`,
			want: []KeyRemapMapping{
				{Src: 30064771129, Dst: 30064771296},
				{Src: 30064771296, Dst: 30064771129},
			},
		},
		{
			name:  "empty array",
			input: `()`,
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			info, err := parseHIDUtilOutput(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if len(info.Mappings) != len(tt.want) {
				t.Fatalf("got %d mappings, want %d", len(info.Mappings), len(tt.want))
			}
			for i, m := range info.Mappings {
				if m != tt.want[i] {
					t.Errorf("mapping[%d] = {Src:%d, Dst:%d}, want {Src:%d, Dst:%d}",
						i, m.Src, m.Dst, tt.want[i].Src, tt.want[i].Dst)
				}
			}
		})
	}
}

func TestParseHIDUtilOutput_EmptyFormats(t *testing.T) {
	t.Parallel()

	for _, input := range []string{"(null)", "()", "(\n)"} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			// These are handled before parseHIDUtilOutput is called,
			// but parseHIDUtilOutput should handle them gracefully if reached.
			info, err := parseHIDUtilOutput(input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(info.Mappings) != 0 {
				t.Errorf("expected no mappings, got %d", len(info.Mappings))
			}
		})
	}
}
