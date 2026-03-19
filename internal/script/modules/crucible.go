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
	_ = obj.Set("file", m.file)
	_ = obj.Set("dir", m.dir)
	_ = obj.Set("symlink", m.symlink)
	_ = obj.Set("brew", m.brew)
	_ = obj.Set("defaults", m.defaults)
	_ = obj.Set("dock", m.dock)
	_ = obj.Set("git", m.git)
	_ = obj.Set("font", m.font)
	_ = obj.Set("mas", m.mas)
	_ = obj.Set("mise", m.mise)
	_ = obj.Set("shell", m.shell)
	_ = obj.Set("log", m.log)
	return obj
}

// isAbsent returns true if the options object has state: "absent".
func (m *CrucibleModule) isAbsent(opts *goja.Object) bool {
	if opts == nil {
		return false
	}
	v := opts.Get("state")
	if v == nil || goja.IsUndefined(v) {
		return false
	}
	return v.String() == "absent"
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
	d := decl.Declaration{
		Type: decl.File,
		Path: path,
		Mode: 0o644, // default
	}

	if len(call.Arguments) >= 2 {
		opts := call.Arguments[1].ToObject(m.vm)
		if m.isAbsent(opts) {
			d.State = decl.Absent
		} else {
			m.applyFileOpts(&d, opts)
		}
	}

	*m.declarations = append(*m.declarations, d)
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
	d := decl.Declaration{
		Type: decl.Dir,
		Path: path,
		Mode: 0o755, // default
	}

	if len(call.Arguments) >= 2 {
		opts := call.Arguments[1].ToObject(m.vm)
		if m.isAbsent(opts) {
			d.State = decl.Absent
		} else if v := opts.Get("mode"); v != nil && !goja.IsUndefined(v) {
			d.Mode = fs.FileMode(v.ToInteger())
		}
	}

	*m.declarations = append(*m.declarations, d)
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

	if m.isAbsent(opts) {
		*m.declarations = append(*m.declarations, decl.Declaration{
			Type:  decl.Symlink,
			Path:  path,
			State: decl.Absent,
		})
		return goja.Undefined()
	}

	targetVal := opts.Get("target")
	if targetVal == nil || goja.IsUndefined(targetVal) {
		panic(m.vm.NewGoError(fmt.Errorf("symlink() requires a target option")))
	}

	*m.declarations = append(*m.declarations, decl.Declaration{
		Type:       decl.Symlink,
		Path:       path,
		LinkTarget: m.expandPath(targetVal.String()),
	})

	return goja.Undefined()
}

// brew declares one or more Homebrew packages.
// Usage: c.brew("coreutils")
//
//	c.brew("ryanwersal/tools/helios")
//	c.brew(["ripgrep", "fd", "bat"])
func (m *CrucibleModule) brew(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) < 1 {
		panic(m.vm.NewGoError(fmt.Errorf("brew() requires a package name argument")))
	}

	var state decl.State
	if len(call.Arguments) >= 2 {
		opts := call.Arguments[1].ToObject(m.vm)
		if m.isAbsent(opts) {
			state = decl.Absent
		}
	}

	exported := call.Arguments[0].Export()
	switch v := exported.(type) {
	case string:
		*m.declarations = append(*m.declarations, decl.Declaration{
			Type:        decl.Package,
			PackageName: v,
			State:       state,
		})
	case []any:
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				panic(m.vm.NewGoError(fmt.Errorf("brew() array elements must be strings")))
			}
			*m.declarations = append(*m.declarations, decl.Declaration{
				Type:        decl.Package,
				PackageName: s,
				State:       state,
			})
		}
	default:
		panic(m.vm.NewGoError(fmt.Errorf("brew() argument must be a string or array of strings")))
	}

	return goja.Undefined()
}

// defaults declares macOS defaults key/value pairs.
// Usage: c.defaults("com.apple.dock", "autohide", true)        // 3-arg form
//
//	c.defaults("com.apple.dock", { autohide: true, tilesize: 36 }) // object form
func (m *CrucibleModule) defaults(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) < 2 {
		panic(m.vm.NewGoError(fmt.Errorf("defaults() requires at least domain and key/object arguments")))
	}

	domain := call.Arguments[0].String()

	// Check if second arg is an object (multi-key form) or string (3-arg form)
	if len(call.Arguments) >= 3 {
		key := call.Arguments[1].String()

		// Check if third arg is an options object with state key
		thirdArg := call.Arguments[2]
		if obj := thirdArg.ToObject(m.vm); obj != nil && m.isAbsent(obj) {
			*m.declarations = append(*m.declarations, decl.Declaration{
				Type:           decl.Defaults,
				DefaultsDomain: domain,
				DefaultsKey:    key,
				State:          decl.Absent,
			})
		} else {
			// 3-arg form: defaults(domain, key, value)
			value := m.coerceDefaultsValue(thirdArg)
			*m.declarations = append(*m.declarations, decl.Declaration{
				Type:           decl.Defaults,
				DefaultsDomain: domain,
				DefaultsKey:    key,
				DefaultsValue:  value,
			})
		}
	} else {
		// Object form: defaults(domain, { key: value, ... })
		obj := call.Arguments[1].ToObject(m.vm)
		for _, key := range obj.Keys() {
			value := m.coerceDefaultsValue(obj.Get(key))
			*m.declarations = append(*m.declarations, decl.Declaration{
				Type:           decl.Defaults,
				DefaultsDomain: domain,
				DefaultsKey:    key,
				DefaultsValue:  value,
			})
		}
	}

	return goja.Undefined()
}

// coerceDefaultsValue converts a goja value to the appropriate Go type for defaults.
func (m *CrucibleModule) coerceDefaultsValue(v goja.Value) any {
	exported := v.Export()
	switch val := exported.(type) {
	case bool:
		return val
	case int64:
		return val
	case float64:
		// If it's a whole number, treat as int64
		if val == float64(int64(val)) {
			return int64(val)
		}
		return val
	default:
		return v.String()
	}
}

// dock declares the desired macOS Dock layout.
// Usage: c.dock({ apps: ["/Applications/Safari.app"], folders: [{ path: "~/Downloads", view: "grid", display: "folder" }] })
func (m *CrucibleModule) dock(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) < 1 {
		panic(m.vm.NewGoError(fmt.Errorf("dock() requires an options argument")))
	}

	opts := call.Arguments[0].ToObject(m.vm)

	d := decl.Declaration{
		Type: decl.Dock,
	}

	if v := opts.Get("apps"); v != nil && !goja.IsUndefined(v) {
		exported := v.Export()
		if arr, ok := exported.([]any); ok {
			for _, item := range arr {
				if s, ok := item.(string); ok {
					d.DockApps = append(d.DockApps, s)
				}
			}
		}
	}

	if v := opts.Get("folders"); v != nil && !goja.IsUndefined(v) {
		exported := v.Export()
		if arr, ok := exported.([]any); ok {
			for _, item := range arr {
				if fm, ok := item.(map[string]any); ok {
					folder := decl.DockFolder{}
					if p, ok := fm["path"].(string); ok {
						folder.Path = m.expandPath(p)
					}
					if view, ok := fm["view"].(string); ok {
						folder.View = view
					}
					if display, ok := fm["display"].(string); ok {
						folder.Display = display
					}
					d.DockFolders = append(d.DockFolders, folder)
				}
			}
		}
	}

	*m.declarations = append(*m.declarations, d)
	return goja.Undefined()
}

// git declares a git repository that should exist at a given path.
// Usage: c.git("~/src/project", { url: "https://github.com/user/repo.git", branch: "main" })
func (m *CrucibleModule) git(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) < 2 {
		panic(m.vm.NewGoError(fmt.Errorf("git() requires path and options arguments")))
	}

	path := m.expandPath(call.Arguments[0].String())
	opts := call.Arguments[1].ToObject(m.vm)

	d := decl.Declaration{
		Type: decl.GitRepo,
		Path: path,
	}

	if v := opts.Get("url"); v != nil && !goja.IsUndefined(v) {
		d.GitURL = v.String()
	}
	if v := opts.Get("branch"); v != nil && !goja.IsUndefined(v) {
		d.GitBranch = v.String()
	}

	*m.declarations = append(*m.declarations, d)
	return goja.Undefined()
}

// font declares font files to install.
// Usage: c.font("fonts/Mono.ttf")                              // single font
//
//	c.font(["fonts/Mono.ttf", "fonts/Sans.otf"])             // array
//	c.font("fonts/Mono.ttf", { dest: "~/Library/Fonts" })   // custom dest
func (m *CrucibleModule) font(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) < 1 {
		panic(m.vm.NewGoError(fmt.Errorf("font() requires a source path argument")))
	}

	destDir := filepath.Join(m.targetDir, "Library", "Fonts")
	var state decl.State

	// Check for options in the last argument
	args := call.Arguments
	if len(args) >= 2 {
		lastArg := args[len(args)-1]
		if obj := lastArg.ToObject(m.vm); obj != nil {
			if m.isAbsent(obj) {
				state = decl.Absent
				args = args[:len(args)-1]
			} else if v := obj.Get("dest"); v != nil && !goja.IsUndefined(v) {
				destDir = m.expandPath(v.String())
				args = args[:len(args)-1]
			}
		}
	}

	// Handle array or single string
	exported := args[0].Export()
	var sources []string
	switch v := exported.(type) {
	case string:
		sources = []string{v}
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				sources = append(sources, s)
			}
		}
	default:
		panic(m.vm.NewGoError(fmt.Errorf("font() argument must be a string or array of strings")))
	}

	for _, src := range sources {
		*m.declarations = append(*m.declarations, decl.Declaration{
			Type:        decl.Font,
			FontSource:  src,
			FontName:    filepath.Base(src),
			FontDestDir: destDir,
			State:       state,
		})
	}

	return goja.Undefined()
}

// mas declares Mac App Store apps to install.
// Usage: c.mas(497799835, "Xcode")          // single app (name optional)
//
//	c.mas([{id: 497799835, name: "Xcode"}, ...]) // array of objects
func (m *CrucibleModule) mas(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) < 1 {
		panic(m.vm.NewGoError(fmt.Errorf("mas() requires an app ID or array argument")))
	}

	exported := call.Arguments[0].Export()
	switch v := exported.(type) {
	case int64:
		m.addMasApp(v, masName(call, 1, m.vm))
	case float64:
		m.addMasApp(int64(v), masName(call, 1, m.vm))
	case []any:
		for _, item := range v {
			fm, ok := item.(map[string]any)
			if !ok {
				panic(m.vm.NewGoError(fmt.Errorf("mas() array elements must be objects with an id field")))
			}
			id, ok := toInt64(fm["id"])
			if !ok {
				panic(m.vm.NewGoError(fmt.Errorf("mas() array element missing numeric id field")))
			}
			name, _ := fm["name"].(string)
			m.addMasApp(id, name)
		}
	default:
		panic(m.vm.NewGoError(fmt.Errorf("mas() argument must be a numeric ID or array of objects")))
	}

	return goja.Undefined()
}

func (m *CrucibleModule) addMasApp(id int64, name string) {
	*m.declarations = append(*m.declarations, decl.Declaration{
		Type:       decl.MasApp,
		MasAppID:   id,
		MasAppName: name,
	})
}

// masName extracts the optional name argument at the given index.
func masName(call goja.FunctionCall, idx int, vm *goja.Runtime) string {
	if len(call.Arguments) > idx {
		v := call.Arguments[idx]
		if !goja.IsUndefined(v) && !goja.IsNull(v) {
			return v.String()
		}
	}
	return ""
}

// toInt64 converts a value that may be float64 or int64 to int64.
func toInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case float64:
		return int64(n), true
	default:
		return 0, false
	}
}

// mise declares globally installed mise tools.
// Usage: c.mise("python", "3.12")
//
//	c.mise("node", "22")
func (m *CrucibleModule) mise(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) < 1 {
		panic(m.vm.NewGoError(fmt.Errorf("mise() requires a tool name argument")))
	}

	name := call.Arguments[0].String()

	if len(call.Arguments) < 2 {
		panic(m.vm.NewGoError(fmt.Errorf("mise() requires a version argument (or { state: \"absent\" })")))
	}

	// Check if second arg is a version string or an options object
	second := call.Arguments[1]
	exported := second.Export()
	if _, isString := exported.(string); isString {
		// mise("python", "3.12") — install form
		*m.declarations = append(*m.declarations, decl.Declaration{
			Type:            decl.MiseTool,
			MiseToolName:    name,
			MiseToolVersion: second.String(),
		})
		return goja.Undefined()
	}

	// Object form: mise("python", { state: "absent" })
	opts := second.ToObject(m.vm)
	if m.isAbsent(opts) {
		*m.declarations = append(*m.declarations, decl.Declaration{
			Type:         decl.MiseTool,
			MiseToolName: name,
			State:        decl.Absent,
		})
		return goja.Undefined()
	}

	panic(m.vm.NewGoError(fmt.Errorf("mise() second argument must be a version string or { state: \"absent\" }")))
}

// shell declares the desired login shell for the current user.
// Usage: c.shell("/opt/homebrew/bin/zsh")
//
//	c.shell("/opt/homebrew/bin/zsh", { user: "ryan" })
func (m *CrucibleModule) shell(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) < 1 {
		panic(m.vm.NewGoError(fmt.Errorf("shell() requires a shell path argument")))
	}

	shellPath := call.Arguments[0].String()
	username := ""

	if len(call.Arguments) >= 2 {
		opts := call.Arguments[1].ToObject(m.vm)
		if v := opts.Get("user"); v != nil && !goja.IsUndefined(v) {
			username = v.String()
		}
	}

	*m.declarations = append(*m.declarations, decl.Declaration{
		Type:          decl.Shell,
		ShellPath:     shellPath,
		ShellUsername:  username,
	})

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
