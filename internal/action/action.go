package action

import (
	"cmp"
	"fmt"
	"io/fs"
	"slices"
	"sync"
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
	SetKeyRemap
	RemoveKeyRemap
	SetDisplay
	RunScript
)

var typeNames sync.Map

// RegisterName records the human-readable name for an action type.
// Called by the resource registry during executor registration.
func RegisterName(t Type, name string) { typeNames.Store(t, name) }

func (t Type) String() string {
	if name, ok := typeNames.Load(t); ok {
		return name.(string)
	}
	return fmt.Sprintf("action(%d)", t)
}

// Action is an inert description of a change to apply.
type Action struct {
	Type              Type
	Group             string // resource group for display; set by engine from decl type (e.g. "File", "Package")
	Path              string
	Description       string
	Recursive         bool         // DeletePath: use os.RemoveAll instead of os.Remove
	Content           []byte       // WriteFile
	Mode              fs.FileMode  // WriteFile, CreateDir, SetPermissions
	LinkTarget        string       // CreateSymlink
	PackageName       string       // InstallPackage
	DefaultsDomain    string       // SetDefaults
	DefaultsKey       string       // SetDefaults
	DefaultsValue     any          // SetDefaults
	DefaultsValueType string       // SetDefaults
	DockApps          []string     // SetDock
	DockFolders       []DockFolder // SetDock
	GitURL            string       // CloneRepo, PullRepo
	GitBranch         string       // CloneRepo, PullRepo
	FontSource        string       // InstallFont: source file path
	FontDest          string       // InstallFont: destination file path
	MiseToolName      string       // InstallMiseTool
	MiseToolVersion   string       // InstallMiseTool
	ShellPath         string       // SetShell
	ShellUsername     string       // SetShell
	MasAppID          int64            // InstallMasApp
	MasAppName        string           // InstallMasApp
	KeyRemaps              []KeyRemapEntry // SetKeyRemap
	DisplaySidebarIconSize string          // SetDisplay: "small", "medium", "large"
	DisplayMenuBarSpacing  string          // SetDisplay: "compact", "default"
	DisplayResolution      string          // SetDisplay: "WxH"
	DisplayHZ              int             // SetDisplay: refresh rate
	ScriptName             string          // RunScript: tool name
	ScriptInstall          string          // RunScript: shell command to run
	NeedsSudo              bool            // action requires privilege escalation
}

// AllTypes returns every registered action Type, sorted by ordinal.
func AllTypes() []Type {
	var types []Type
	typeNames.Range(func(key, _ any) bool {
		types = append(types, key.(Type))
		return true
	})
	slices.SortFunc(types, cmp.Compare)
	return types
}

// KeyRemapEntry describes a single key remapping (from → to).
type KeyRemapEntry struct {
	From string
	To   string
}

// DockFolder describes a folder entry in the Dock.
type DockFolder struct {
	Path    string
	View    string
	Display string
}
