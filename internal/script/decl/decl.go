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

// DockFolder describes a folder entry in the Dock declaration.
type DockFolder struct {
	Path    string
	View    string
	Display string
}
