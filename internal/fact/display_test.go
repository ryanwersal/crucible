package fact

import "testing"

func TestParseResolution(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantW   int
		wantH   int
		wantErr bool
	}{
		{
			name:  "standard resolution",
			input: "1800x1169",
			wantW: 1800,
			wantH: 1169,
		},
		{
			name:  "4K resolution",
			input: "3840x2160",
			wantW: 3840,
			wantH: 2160,
		},
		{
			name:    "missing height",
			input:   "1800",
			wantErr: true,
		},
		{
			name:    "invalid width",
			input:   "abcx1169",
			wantErr: true,
		},
		{
			name:    "invalid height",
			input:   "1800xabc",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w, h, err := parseResolution(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if w != tt.wantW {
				t.Errorf("width = %d, want %d", w, tt.wantW)
			}
			if h != tt.wantH {
				t.Errorf("height = %d, want %d", h, tt.wantH)
			}
		})
	}
}
