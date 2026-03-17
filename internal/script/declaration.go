package script

// Declaration types and the Declaration struct live in internal/script/decl
// to avoid import cycles between script and script/modules.
// This file re-exports them for convenience.

import "github.com/ryanwersal/crucible/internal/script/decl"

// Re-export declaration types for external consumers.
type (
	Declaration     = decl.Declaration
	DeclarationType = decl.Type
)

const (
	DeclFile    = decl.File
	DeclDir     = decl.Dir
	DeclSymlink = decl.Symlink
	DeclPackage = decl.Package
)
