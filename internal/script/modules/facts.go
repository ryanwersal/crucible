package modules

import (
	"context"
	"fmt"

	"github.com/dop251/goja"

	"github.com/ryanwersal/crucible/internal/fact"
)

// FactsModule implements the "crucible/facts" native module that exposes
// system facts to JavaScript. OS and homebrew facts are pre-collected;
// file and dir facts are collected on demand.
type FactsModule struct {
	vm    *goja.Runtime
	ctx   context.Context
	store *fact.Store
}

// NewFactsModule creates the facts module with a pre-populated store.
func NewFactsModule(vm *goja.Runtime, ctx context.Context, store *fact.Store) *FactsModule {
	return &FactsModule{vm: vm, ctx: ctx, store: store}
}

// Export returns a goja.Object exposing the facts API.
func (m *FactsModule) Export() *goja.Object {
	obj := m.vm.NewObject()
	_ = obj.Set("os", m.osObject())
	_ = obj.Set("homebrew", m.homebrewObject())
	_ = obj.Set("file", m.fileFunc())
	_ = obj.Set("dir", m.dirFunc())
	return obj
}

func (m *FactsModule) osObject() *goja.Object {
	obj := m.vm.NewObject()

	// OS facts are pre-collected so Get will return the cached value.
	osInfo, err := fact.Get(m.ctx, m.store, "os", fact.OSCollector{})
	if err != nil {
		// Return empty object if OS facts unavailable
		return obj
	}

	_ = obj.Set("name", osInfo.OS)
	_ = obj.Set("arch", osInfo.Arch)
	_ = obj.Set("hostname", osInfo.Hostname)
	return obj
}

func (m *FactsModule) homebrewObject() *goja.Object {
	obj := m.vm.NewObject()

	brewInfo, err := fact.Get(m.ctx, m.store, "homebrew", fact.HomebrewCollector{})
	if err != nil {
		_ = obj.Set("available", false)
		return obj
	}

	_ = obj.Set("available", brewInfo.Available)

	formulae := make([]string, 0, len(brewInfo.Formulae))
	for name := range brewInfo.Formulae {
		formulae = append(formulae, name)
	}
	_ = obj.Set("formulae", formulae)

	casks := make([]string, 0, len(brewInfo.Casks))
	for name := range brewInfo.Casks {
		casks = append(casks, name)
	}
	_ = obj.Set("casks", casks)
	return obj
}

func (m *FactsModule) fileFunc() func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(m.vm.NewGoError(fmt.Errorf("facts.file() requires a path argument")))
		}

		path := call.Arguments[0].String()
		key := "file:" + path

		fi, err := fact.Get(m.ctx, m.store, key, fact.FileCollector{Path: path})
		if err != nil {
			panic(m.vm.NewGoError(err))
		}

		obj := m.vm.NewObject()
		_ = obj.Set("exists", fi.Exists)
		_ = obj.Set("hash", fi.Hash)
		_ = obj.Set("mode", int64(fi.Mode))
		_ = obj.Set("size", fi.Size)
		_ = obj.Set("isDir", fi.IsDir)
		_ = obj.Set("isLink", fi.IsLink)
		return obj
	}
}

func (m *FactsModule) dirFunc() func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(m.vm.NewGoError(fmt.Errorf("facts.dir() requires a path argument")))
		}

		path := call.Arguments[0].String()
		key := "dir:" + path

		di, err := fact.Get(m.ctx, m.store, key, fact.DirCollector{Path: path})
		if err != nil {
			panic(m.vm.NewGoError(err))
		}

		obj := m.vm.NewObject()
		_ = obj.Set("exists", di.Exists)
		_ = obj.Set("mode", int64(di.Mode))
		_ = obj.Set("children", di.Children)
		return obj
	}
}
