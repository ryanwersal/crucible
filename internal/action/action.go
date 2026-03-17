package action

import "io/fs"

// Type identifies the kind of action to perform.
type Type int

const (
	WriteFile Type = iota
	CreateDir
	CreateSymlink
	SetPermissions
	DeletePath
	InstallPackage
)

func (t Type) String() string {
	switch t {
	case WriteFile:
		return "WriteFile"
	case CreateDir:
		return "CreateDir"
	case CreateSymlink:
		return "CreateSymlink"
	case SetPermissions:
		return "SetPermissions"
	case DeletePath:
		return "DeletePath"
	case InstallPackage:
		return "InstallPackage"
	default:
		return "Unknown"
	}
}

// Action is an inert description of a change to apply.
type Action struct {
	Type        Type
	Path        string
	Description string
	Content     []byte      // WriteFile
	Mode        fs.FileMode // WriteFile, CreateDir, SetPermissions
	LinkTarget  string      // CreateSymlink
	PackageName string      // InstallPackage
}
