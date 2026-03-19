package action

import (
	"cmp"
	"fmt"
	"io/fs"
	"slices"
)

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
	InstallMasApp
)

var typeNames = map[Type]string{}

// RegisterName records the human-readable name for an action type.
// Called by the resource registry during executor registration.
func RegisterName(t Type, name string) { typeNames[t] = name }

func (t Type) String() string {
	if name, ok := typeNames[t]; ok {
		return name
	}
	return fmt.Sprintf("action(%d)", t)
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
	MasAppID          int64       // InstallMasApp
	MasAppName        string      // InstallMasApp
	NeedsSudo         bool        // action requires privilege escalation
}

// AllTypes returns every registered action Type, sorted by ordinal.
func AllTypes() []Type {
	types := make([]Type, 0, len(typeNames))
	for t := range typeNames {
		types = append(types, t)
	}
	slices.SortFunc(types, func(a, b Type) int { return cmp.Compare(a, b) })
	return types
}

// DockFolder describes a folder entry in the Dock.
type DockFolder struct {
	Path    string
	View    string
	Display string
}
