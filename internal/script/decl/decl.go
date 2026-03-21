package decl

import (
	"cmp"
	"fmt"
	"io/fs"
	"slices"
)

// State indicates whether a declaration should be present or absent.
type State int

const (
	Present State = iota // zero value = backward compatible
	Absent
)

// Type identifies what a script declaration manages.
type Type int

const (
	File Type = iota
	Dir
	Symlink
	Package
	Defaults
	Dock
	GitRepo
	Font
	MiseTool
	Shell
	MasApp
	KeyRemap
)

var typeNames = map[Type]string{}

// RegisterName records the human-readable name for a declaration type.
// Called by the resource registry during handler registration.
func RegisterName(t Type, name string) { typeNames[t] = name }

func (t Type) String() string {
	if name, ok := typeNames[t]; ok {
		return name
	}
	return fmt.Sprintf("decl(%d)", t)
}

// Declaration represents a single desired-state entry produced by a script.
type Declaration struct {
	Type            Type
	State           State
	Path            string         // target path (~ expanded)
	Content         []byte         // File: inline content
	SourceFile      string         // File: relative path in source dir
	TemplateFile    string         // File: relative path to .tmpl in source dir
	TemplateData    map[string]any // File: template variables
	Mode            fs.FileMode    // File, Dir
	LinkTarget      string         // Symlink
	PackageName     string         // Package
	DefaultsDomain  string         // Defaults
	DefaultsKey     string         // Defaults
	DefaultsValue   any            // Defaults
	DockApps        []string       // Dock
	DockFolders     []DockFolder   // Dock
	GitURL          string         // GitRepo
	GitBranch       string         // GitRepo
	FontSource      string         // Font: relative path to font file in source dir
	FontName        string         // Font: filename (e.g. "Mono.ttf")
	FontDestDir     string         // Font: destination directory
	MiseToolName    string         // MiseTool
	MiseToolVersion string         // MiseTool
	ShellPath       string         // Shell
	ShellUsername   string         // Shell
	MasAppID        int64          // MasApp
	MasAppName      string         // MasApp
	KeyRemaps       []KeyRemapEntry // KeyRemap
}

// AllTypes returns every registered declaration Type, sorted by ordinal.
func AllTypes() []Type {
	types := make([]Type, 0, len(typeNames))
	for t := range typeNames {
		types = append(types, t)
	}
	slices.SortFunc(types, cmp.Compare)
	return types
}

// KeyRemapEntry describes a single key remapping (from → to).
type KeyRemapEntry struct {
	From string
	To   string
}

// hidKeyCodes maps human-readable key names to HID usage page key codes.
// These are USB HID usage codes with the 0x700000000 usage page prefix.
var hidKeyCodes = map[string]uint64{
	"capsLock":     0x700000039,
	"control":      0x7000000E0,
	"leftControl":  0x7000000E0,
	"rightControl": 0x7000000E4,
	"leftShift":    0x7000000E1,
	"rightShift":   0x7000000E5,
	"leftOption":   0x7000000E2,
	"rightOption":  0x7000000E6,
	"leftCommand":  0x7000000E3,
	"rightCommand": 0x7000000E7,
	"fn":           0xFF00000003,
}

// ValidKeyName reports whether name is a recognized key name for remapping.
func ValidKeyName(name string) bool {
	_, ok := hidKeyCodes[name]
	return ok
}

// KeyCode returns the HID usage code for the given key name.
func KeyCode(name string) (uint64, bool) {
	code, ok := hidKeyCodes[name]
	return code, ok
}

// ValidKeyNames returns all recognized key names, sorted.
func ValidKeyNames() []string {
	names := make([]string, 0, len(hidKeyCodes))
	for name := range hidKeyCodes {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// DockFolder describes a folder entry in the Dock declaration.
type DockFolder struct {
	Path    string
	View    string
	Display string
}
