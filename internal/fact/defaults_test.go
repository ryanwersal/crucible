package fact

import "testing"

func TestParseDefaultsOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		typeOutput string
		valOutput  string
		want       any
		wantErr    bool
	}{
		{
			name:       "boolean true",
			typeOutput: "Type is boolean",
			valOutput:  "1",
			want:       true,
		},
		{
			name:       "boolean false",
			typeOutput: "Type is boolean",
			valOutput:  "0",
			want:       false,
		},
		{
			name:       "integer",
			typeOutput: "Type is integer",
			valOutput:  "36",
			want:       int64(36),
		},
		{
			name:       "float",
			typeOutput: "Type is float",
			valOutput:  "1.5",
			want:       1.5,
		},
		{
			name:       "string",
			typeOutput: "Type is string",
			valOutput:  "hello world",
			want:       "hello world",
		},
		{
			name:       "unsupported type",
			typeOutput: "Type is data",
			valOutput:  "abc",
			wantErr:    true,
		},
		{
			name:       "bad boolean",
			typeOutput: "Type is boolean",
			valOutput:  "yes",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseDefaultsOutput(tt.typeOutput, tt.valOutput)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("got %v (%T), want %v (%T)", got, got, tt.want, tt.want)
			}
		})
	}
}
