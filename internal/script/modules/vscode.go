package modules

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/dop251/goja"
	"github.com/ryanwersal/crucible/internal/script/decl"
)

var extensionID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*\.[A-Za-z0-9][A-Za-z0-9-]*$`)

func (m *CrucibleModule) vscode(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) < 1 || len(call.Arguments) > 2 {
		panic(m.vm.NewGoError(fmt.Errorf("vscode() requires extension IDs and optional options")))
	}
	state := decl.Present
	if len(call.Arguments) == 2 {
		opts, ok := call.Arguments[1].(*goja.Object)
		if !ok || opts.ClassName() != "Object" {
			panic(m.vm.NewGoError(fmt.Errorf("vscode() options must be an object")))
		}
		for _, key := range opts.Keys() {
			if key != "state" {
				panic(m.vm.NewGoError(fmt.Errorf("vscode() unknown option %q", key)))
			}
		}
		switch value := optString(opts, "state"); value {
		case "", "present":
		case "absent":
			state = decl.Absent
		default:
			panic(m.vm.NewGoError(fmt.Errorf("vscode() unknown state %q; valid values: present, absent", value)))
		}
	}
	var ids []string
	switch value := call.Arguments[0].Export().(type) {
	case string:
		ids = []string{value}
	case []any:
		for _, item := range value {
			id, ok := item.(string)
			if !ok {
				panic(m.vm.NewGoError(fmt.Errorf("vscode() extension IDs must be strings")))
			}
			ids = append(ids, id)
		}
	default:
		panic(m.vm.NewGoError(fmt.Errorf("vscode() requires a string or array of strings")))
	}
	for _, id := range ids {
		if !extensionID.MatchString(id) {
			panic(m.vm.NewGoError(fmt.Errorf("vscode() invalid extension ID %q; expected publisher.name", id)))
		}
	}
	for _, id := range ids {
		*m.declarations = append(*m.declarations, decl.Declaration{
			Type: decl.VSCodeExtension, VSCodeExtension: strings.ToLower(id), State: state,
		})
	}
	return goja.Undefined()
}
