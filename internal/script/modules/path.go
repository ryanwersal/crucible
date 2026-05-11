package modules

import (
	"fmt"
	"path/filepath"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
)

// RegisterPath registers the Node-compatible path module under both
// "node:path" and "path", matching how Node resolves either specifier
// to the same built-in.
//
// Currently only join() is implemented; additional functions can be
// added on the same module object as needed.
func RegisterPath(registry *require.Registry) {
	loader := func(vm *goja.Runtime, module *goja.Object) {
		exports := module.Get("exports").(*goja.Object)
		_ = exports.Set("join", pathJoin(vm))
	}
	registry.RegisterNativeModule("node:path", loader)
	registry.RegisterNativeModule("path", loader)
}

// pathJoin returns a JS function implementing Node's path.join semantics:
// joins all arguments with the platform separator, normalizes the result,
// and returns "." for an empty join. Throws TypeError if any argument is
// not a string, matching Node's runtime check.
func pathJoin(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		segments := make([]string, 0, len(call.Arguments))
		for i, arg := range call.Arguments {
			exported := arg.Export()
			s, ok := exported.(string)
			if !ok {
				panic(vm.NewTypeError(fmt.Sprintf(
					"Path must be a string. Received %s at index %d",
					describeJSType(exported), i,
				)))
			}
			segments = append(segments, s)
		}
		if len(segments) == 0 {
			return vm.ToValue(".")
		}
		return vm.ToValue(filepath.Join(segments...))
	}
}

// describeJSType produces a Node-ish description of a non-string value
// for the TypeError message.
func describeJSType(v any) string {
	switch v.(type) {
	case nil:
		return "undefined"
	case bool:
		return "type boolean"
	case int64, float64:
		return "type number"
	default:
		return fmt.Sprintf("type %T", v)
	}
}
