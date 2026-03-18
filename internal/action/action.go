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
	SetDefaults
	SetDock
	CloneRepo
	PullRepo
	InstallFont
	InstallMiseTool
	SetShell
	UninstallPackage
	UninstallMiseTool
	DeleteDefaults
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
	case SetDefaults:
		return "SetDefaults"
	case SetDock:
		return "SetDock"
	case CloneRepo:
		return "CloneRepo"
	case PullRepo:
		return "PullRepo"
	case InstallFont:
		return "InstallFont"
	case InstallMiseTool:
		return "InstallMiseTool"
	case SetShell:
		return "SetShell"
	case UninstallPackage:
		return "UninstallPackage"
	case UninstallMiseTool:
		return "UninstallMiseTool"
	case DeleteDefaults:
		return "DeleteDefaults"
	default:
		return "Unknown"
	}
}

// Action is an inert description of a change to apply.
type Action struct {
	Type        Type
	Path        string
	Description string
	Recursive   bool        // DeletePath: use os.RemoveAll instead of os.Remove
	Content     []byte      // WriteFile
	Mode        fs.FileMode // WriteFile, CreateDir, SetPermissions
	LinkTarget  string      // CreateSymlink
	PackageName       string      // InstallPackage
	DefaultsDomain    string      // SetDefaults
	DefaultsKey       string      // SetDefaults
	DefaultsValue     any         // SetDefaults
	DefaultsValueType string      // SetDefaults
	DockApps          []string    // SetDock
	DockFolders       []DockFolder // SetDock
	GitURL            string      // CloneRepo, PullRepo
	GitBranch         string      // CloneRepo, PullRepo
	FontSource        string      // InstallFont: source file path
	FontDest          string      // InstallFont: destination file path
	MiseToolName      string      // InstallMiseTool
	MiseToolVersion   string      // InstallMiseTool
	ShellPath         string      // SetShell
	ShellUsername     string      // SetShell
}

// DockFolder describes a folder entry in the Dock.
type DockFolder struct {
	Path    string
	View    string
	Display string
}
