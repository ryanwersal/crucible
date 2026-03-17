package decl

import "testing"

func TestType_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		dt   Type
		want string
	}{
		{File, "File"},
		{Dir, "Dir"},
		{Symlink, "Symlink"},
		{Package, "Package"},
		{Type(99), "Unknown"},
	}

	for _, tt := range tests {
		if got := tt.dt.String(); got != tt.want {
			t.Errorf("Type(%d).String() = %q, want %q", tt.dt, got, tt.want)
		}
	}
}
