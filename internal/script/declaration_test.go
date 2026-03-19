package script

import (
	"testing"

	"github.com/ryanwersal/crucible/internal/script/decl"
)

func TestDeclarationType_String(t *testing.T) {
	t.Parallel()

	// Register names as the registry would.
	decl.RegisterName(decl.File, "File")
	decl.RegisterName(decl.Dir, "Dir")
	decl.RegisterName(decl.Symlink, "Symlink")
	decl.RegisterName(decl.Package, "Package")

	tests := []struct {
		dt   DeclarationType
		want string
	}{
		{DeclFile, "File"},
		{DeclDir, "Dir"},
		{DeclSymlink, "Symlink"},
		{DeclPackage, "Package"},
		{DeclarationType(99), "decl(99)"},
	}

	for _, tt := range tests {
		if got := tt.dt.String(); got != tt.want {
			t.Errorf("DeclarationType(%d).String() = %q, want %q", tt.dt, got, tt.want)
		}
	}
}
