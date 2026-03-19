package decl

import "testing"

func TestType_String(t *testing.T) {
	t.Parallel()

	// Register names as the registry would.
	RegisterName(File, "File")
	RegisterName(Dir, "Dir")
	RegisterName(Symlink, "Symlink")
	RegisterName(Package, "Package")

	tests := []struct {
		dt   Type
		want string
	}{
		{File, "File"},
		{Dir, "Dir"},
		{Symlink, "Symlink"},
		{Package, "Package"},
		{Type(99), "decl(99)"},
	}

	for _, tt := range tests {
		if got := tt.dt.String(); got != tt.want {
			t.Errorf("Type(%d).String() = %q, want %q", tt.dt, got, tt.want)
		}
	}
}
