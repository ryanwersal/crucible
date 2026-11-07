package modules

import (
	"fmt"

	"github.com/dop251/goja"
	"github.com/ryanwersal/crucible/internal/script/decl"
	"github.com/ryanwersal/crucible/internal/softwareupdate"
)

func (m *CrucibleModule) softwareUpdate(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) < 1 || len(call.Arguments) > 2 {
		panic(m.vm.NewGoError(fmt.Errorf("softwareUpdate(): requires an update name and optional options")))
	}
	name, ok := call.Argument(0).Export().(string)
	if !ok {
		panic(m.vm.NewGoError(fmt.Errorf("softwareUpdate(): name must be a string")))
	}
	return m.declareSoftwareUpdate(softwareupdate.Request{Name: name, Match: "title", Latest: true}, call.Argument(1))
}

func (m *CrucibleModule) commandLineTools(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) > 1 {
		panic(m.vm.NewGoError(fmt.Errorf("softwareUpdate.commandLineTools(): accepts only options")))
	}
	return m.declareSoftwareUpdate(softwareupdate.Request{Name: "Command Line Tools for Xcode", Match: "prefix", Receipt: softwareupdate.CLTReceipt, CommandLineTools: true}, call.Argument(0))
}

func (m *CrucibleModule) declareSoftwareUpdate(request softwareupdate.Request, options goja.Value) goja.Value {
	fail := func(message string) { panic(m.vm.NewGoError(fmt.Errorf("softwareUpdate(): %s", message))) }
	if options != nil && !goja.IsUndefined(options) {
		object, ok := options.(*goja.Object)
		if !ok || object.ClassName() != "Object" {
			fail("options must be an object")
		}
		for _, key := range object.Keys() {
			value := object.Get(key)
			if key == "restart" {
				flag, ok := value.Export().(bool)
				if !ok {
					fail("restart must be a boolean")
				}
				request.Restart = flag
				continue
			}
			text, ok := value.Export().(string)
			if !ok {
				fail(key + " must be a string")
			}
			switch key {
			case "state":
				if text != "present" && text != "latest" {
					fail("state must be present or latest; Apple updates cannot be uninstalled")
				}
				request.Latest = text == "latest"
			case "match":
				if request.CommandLineTools {
					fail("commandLineTools manages its own selector")
				}
				request.Match = text
			case "receipt":
				if request.CommandLineTools {
					fail("commandLineTools manages its own receipt")
				}
				request.Receipt = text
			default:
				fail("unknown option " + key)
			}
		}
	}
	if err := request.Validate(); err != nil {
		fail(err.Error())
	}
	for _, existing := range *m.declarations {
		if existing.Type != decl.SoftwareUpdate {
			continue
		}
		other := existing.SoftwareUpdate
		if other.Name == request.Name && other.Match == request.Match {
			if other != request {
				fail("conflicting declarations for " + request.Name)
			}
			return goja.Undefined()
		}
	}
	state := decl.Present
	if request.Latest {
		state = decl.Latest
	}
	*m.declarations = append(*m.declarations, decl.Declaration{Type: decl.SoftwareUpdate, State: state, SoftwareUpdate: request})
	return goja.Undefined()
}
