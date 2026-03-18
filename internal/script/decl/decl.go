package decl

import "io/fs"

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
)

func (d Type) String() string {
	switch d {
	case File:
		return "File"
	case Dir:
		return "Dir"
	case Symlink:
		return "Symlink"
	case Package:
		return "Package"
	case Defaults:
		return "Defaults"
	case Dock:
		return "Dock"
	case GitRepo:
		return "GitRepo"
	case Font:
		return "Font"
	case MiseTool:
		return "MiseTool"
	case Shell:
		return "Shell"
	default:
		return "Unknown"
	}
}

// Declaration represents a single desired-state entry produced by a script.
type Declaration struct {
	Type         Type
	Path         string         // target path (~ expanded)
	Content      []byte         // File: inline content
	SourceFile   string         // File: relative path in source dir
	TemplateFile string         // File: relative path to .tmpl in source dir
	TemplateData map[string]any // File: template variables
	Mode         fs.FileMode    // File, Dir
	LinkTarget     string         // Symlink
	PackageName    string         // Package
	DefaultsDomain string         // Defaults
	DefaultsKey    string         // Defaults
	DefaultsValue  any            // Defaults
	DockApps       []string       // Dock
	DockFolders    []DockFolder   // Dock
	GitURL         string         // GitRepo
	GitBranch      string         // GitRepo
	FontSource     string         // Font: relative path to font file in source dir
	FontName       string         // Font: filename (e.g. "Mono.ttf")
	FontDestDir    string         // Font: destination directory
	MiseToolName    string        // MiseTool
	MiseToolVersion string        // MiseTool
	ShellPath       string        // Shell
	ShellUsername   string        // Shell
}

// DockFolder describes a folder entry in the Dock declaration.
type DockFolder struct {
	Path    string
	View    string
	Display string
}
