package script

import "testing"

func TestDeclarationType_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		dt   DeclarationType
		want string
	}{
		{DeclFile, "File"},
		{DeclDir, "Dir"},
		{DeclSymlink, "Symlink"},
		{DeclPackage, "Package"},
		{DeclarationType(99), "Unknown"},
	}

	for _, tt := range tests {
		if got := tt.dt.String(); got != tt.want {
			t.Errorf("DeclarationType(%d).String() = %q, want %q", tt.dt, got, tt.want)
		}
	}
}
