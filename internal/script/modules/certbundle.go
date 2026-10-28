package modules

import (
	"fmt"
	"path/filepath"

	"github.com/dop251/goja"
	"github.com/ryanwersal/crucible/internal/certbundle"
	"github.com/ryanwersal/crucible/internal/script/decl"
)

func (m *CrucibleModule) certBundle(call goja.FunctionCall) goja.Value {
	fail := func(message string) { panic(m.vm.NewGoError(fmt.Errorf("certBundle(): %s", message))) }
	if len(call.Arguments) != 2 {
		fail("requires an absolute path and options")
	}
	path, ok := call.Argument(0).Export().(string)
	if !ok || path == "" {
		fail("path must be a nonempty string")
	}
	path = m.expandPath(path)
	if !filepath.IsAbs(path) {
		fail("path must be absolute (or begin with ~/)")
	}
	opts, ok := call.Argument(1).(*goja.Object)
	if !ok || opts.ClassName() != "Object" {
		fail("options must be an object")
	}
	for _, key := range opts.Keys() {
		if key != "absent" && key != "sudo" {
			fail("unknown option " + key)
		}
	}
	value := opts.Get("absent")
	array, ok := value.(*goja.Object)
	if !ok || array.ClassName() != "Array" {
		fail("absent must be an array of SHA-256 fingerprints")
	}
	var fingerprints []string
	if err := m.vm.ExportTo(value, &fingerprints); err != nil {
		fail("absent must contain strings")
	}
	// ExportTo can coerce array entries. Require actual strings before validation.
	for i := int64(0); i < array.Get("length").ToInteger(); i++ {
		entry := array.Get(fmt.Sprint(i))
		if entry == nil {
			fail("absent must contain strings")
		}
		if _, ok := entry.Export().(string); !ok {
			fail("absent must contain strings")
		}
	}
	fingerprints, err := certbundle.NormalizeFingerprints(fingerprints)
	if err != nil {
		fail(err.Error())
	}
	sudo := false
	if option := m.optBoolPtr(opts, "sudo"); option != nil {
		sudo = *option
	}
	for _, existing := range *m.declarations {
		if existing.Path == path {
			fail("path already declared: " + path)
		}
	}
	*m.declarations = append(*m.declarations, decl.Declaration{Type: decl.CertBundle, Path: path, CertAbsent: fingerprints, CertSudo: sudo})
	return goja.Undefined()
}
