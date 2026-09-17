package modules

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/dop251/goja"
	"github.com/ryanwersal/crucible/internal/script/decl"
)

var tapNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]*/[a-z0-9][a-z0-9_.-]*$`)

func (m *CrucibleModule) brewTap(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) == 0 {
		panic(m.vm.NewGoError(fmt.Errorf("brew.tap() requires an owner/repository name")))
	}
	name, ok := call.Arguments[0].Export().(string)
	if !ok {
		panic(m.vm.NewGoError(fmt.Errorf("brew.tap() requires a string name")))
	}
	name = strings.ToLower(name)
	if !tapNamePattern.MatchString(name) {
		panic(m.vm.NewGoError(fmt.Errorf("brew.tap(): invalid tap name %q; use owner/repository", name)))
	}
	parts := strings.SplitN(name, "/", 2)
	parts[1] = strings.TrimPrefix(parts[1], "homebrew-")
	if parts[1] == "" {
		panic(m.vm.NewGoError(fmt.Errorf("brew.tap(): repository name is empty")))
	}
	name = strings.Join(parts, "/")
	state := decl.Present
	if len(call.Arguments) > 1 {
		opts := call.Arguments[1].ToObject(m.vm)
		if v := opts.Get("state"); v != nil && !goja.IsUndefined(v) {
			switch v.String() {
			case "present":
			case "absent":
				state = decl.Absent
			default:
				panic(m.vm.NewGoError(fmt.Errorf("brew.tap(): state must be present or absent")))
			}
		}
	}
	*m.declarations = append(*m.declarations, decl.Declaration{Type: decl.HomebrewTap, TapName: name, State: state})
	return goja.Undefined()
}

func (m *CrucibleModule) brewCask(call goja.FunctionCall) goja.Value {
	return m.typedBrew(call, "cask")
}

func (m *CrucibleModule) brewFormula(call goja.FunctionCall) goja.Value {
	return m.typedBrew(call, "formula")
}

func (m *CrucibleModule) typedBrew(call goja.FunctionCall, kind string) goja.Value {
	start := len(*m.declarations)
	result := m.brew(call)
	for i := start; i < len(*m.declarations); i++ {
		(*m.declarations)[i].PackageKind = kind
	}
	return result
}
