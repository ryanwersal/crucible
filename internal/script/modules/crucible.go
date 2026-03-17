package modules

import (
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/dop251/goja"

	"github.com/ryanwersal/crucible/internal/script/decl"
)

// CrucibleModule implements the "crucible" native module that provides
// file(), dir(), symlink(), brew(), and log() functions for declaring
// desired system state.
type CrucibleModule struct {
	vm           *goja.Runtime
	logger       *slog.Logger
	targetDir    string
	declarations *[]decl.Declaration
}

// NewCrucibleModule creates the crucible host API module.
func NewCrucibleModule(vm *goja.Runtime, logger *slog.Logger, targetDir string, declarations *[]decl.Declaration) *CrucibleModule {
	return &CrucibleModule{
		vm:           vm,
		logger:       logger,
		targetDir:    targetDir,
		declarations: declarations,
	}
}

// Export returns a goja.Object exposing the module's API.
func (m *CrucibleModule) Export() *goja.Object {
	obj := m.vm.NewObject()
	obj.Set("file", m.file)
	obj.Set("dir", m.dir)
	obj.Set("symlink", m.symlink)
	obj.Set("brew", m.brew)
	obj.Set("log", m.log)
	return obj
}

// expandPath resolves ~ to the target directory and cleans the path.
func (m *CrucibleModule) expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		path = filepath.Join(m.targetDir, path[2:])
	} else if path == "~" {
		path = m.targetDir
	}
	return filepath.Clean(path)
}

// file declares a managed file.
// Usage: c.file("~/.gitconfig", { content: "...", mode: 0o644 })
//
//	c.file("~/.config/fish/config.fish", { source: "fish/config.fish" })
//	c.file("~/.config/starship.toml", { template: "starship.toml.tmpl", data: { ... } })
func (m *CrucibleModule) file(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) < 1 {
		panic(m.vm.NewGoError(fmt.Errorf("file() requires a path argument")))
	}

	path := m.expandPath(call.Arguments[0].String())
	decl := decl.Declaration{
		Type: decl.File,
		Path: path,
		Mode: 0o644, // default
	}

	if len(call.Arguments) >= 2 {
		opts := call.Arguments[1].ToObject(m.vm)
		m.applyFileOpts(&decl, opts)
	}

	*m.declarations = append(*m.declarations, decl)
	return goja.Undefined()
}

func (m *CrucibleModule) applyFileOpts(decl *decl.Declaration, opts *goja.Object) {
	if v := opts.Get("content"); v != nil && !goja.IsUndefined(v) {
		decl.Content = []byte(v.String())
	}
	if v := opts.Get("source"); v != nil && !goja.IsUndefined(v) {
		decl.SourceFile = v.String()
	}
	if v := opts.Get("template"); v != nil && !goja.IsUndefined(v) {
		decl.TemplateFile = v.String()
	}
	if v := opts.Get("data"); v != nil && !goja.IsUndefined(v) {
		decl.TemplateData = exportToMap(v, m.vm)
	}
	if v := opts.Get("mode"); v != nil && !goja.IsUndefined(v) {
		decl.Mode = fs.FileMode(v.ToInteger())
	}
}

// dir declares a managed directory.
// Usage: c.dir("~/.config/fish", { mode: 0o755 })
func (m *CrucibleModule) dir(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) < 1 {
		panic(m.vm.NewGoError(fmt.Errorf("dir() requires a path argument")))
	}

	path := m.expandPath(call.Arguments[0].String())
	decl := decl.Declaration{
		Type: decl.Dir,
		Path: path,
		Mode: 0o755, // default
	}

	if len(call.Arguments) >= 2 {
		opts := call.Arguments[1].ToObject(m.vm)
		if v := opts.Get("mode"); v != nil && !goja.IsUndefined(v) {
			decl.Mode = fs.FileMode(v.ToInteger())
		}
	}

	*m.declarations = append(*m.declarations, decl)
	return goja.Undefined()
}

// symlink declares a managed symlink.
// Usage: c.symlink("~/.vimrc", { target: "~/.config/nvim/init.vim" })
func (m *CrucibleModule) symlink(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) < 2 {
		panic(m.vm.NewGoError(fmt.Errorf("symlink() requires path and options arguments")))
	}

	path := m.expandPath(call.Arguments[0].String())
	opts := call.Arguments[1].ToObject(m.vm)

	targetVal := opts.Get("target")
	if targetVal == nil || goja.IsUndefined(targetVal) {
		panic(m.vm.NewGoError(fmt.Errorf("symlink() requires a target option")))
	}

	decl := decl.Declaration{
		Type:       decl.Symlink,
		Path:       path,
		LinkTarget: m.expandPath(targetVal.String()),
	}

	*m.declarations = append(*m.declarations, decl)
	return goja.Undefined()
}

// brew declares a Homebrew package.
// Usage: c.brew("coreutils")
//
//	c.brew("ryanwersal/tools/helios")
func (m *CrucibleModule) brew(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) < 1 {
		panic(m.vm.NewGoError(fmt.Errorf("brew() requires a package name argument")))
	}

	d := decl.Declaration{
		Type:        decl.Package,
		PackageName: call.Arguments[0].String(),
	}

	*m.declarations = append(*m.declarations, d)
	return goja.Undefined()
}

// log outputs a message via slog.
// Usage: c.log("installing packages...")
func (m *CrucibleModule) log(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) < 1 {
		return goja.Undefined()
	}

	msg := call.Arguments[0].String()
	m.logger.Info(msg, "source", "script")
	return goja.Undefined()
}

// exportToMap converts a goja value to a Go map[string]any.
func exportToMap(v goja.Value, vm *goja.Runtime) map[string]any {
	exported := v.Export()
	if m, ok := exported.(map[string]any); ok {
		return m
	}
	// Try converting from goja object
	obj := v.ToObject(vm)
	result := make(map[string]any)
	for _, key := range obj.Keys() {
		result[key] = obj.Get(key).Export()
	}
	return result
}
