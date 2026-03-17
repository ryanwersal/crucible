package decl

import "io/fs"

// Type identifies what a script declaration manages.
type Type int

const (
	File Type = iota
	Dir
	Symlink
	Package
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
	LinkTarget   string         // Symlink
	PackageName  string         // Package
}
