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
	obj.Set("os", m.osObject())
	obj.Set("homebrew", m.homebrewObject())
	obj.Set("file", m.fileFunc())
	obj.Set("dir", m.dirFunc())
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

	obj.Set("name", osInfo.OS)
	obj.Set("arch", osInfo.Arch)
	obj.Set("hostname", osInfo.Hostname)
	return obj
}

func (m *FactsModule) homebrewObject() *goja.Object {
	obj := m.vm.NewObject()

	brewInfo, err := fact.Get(m.ctx, m.store, "homebrew", fact.HomebrewCollector{})
	if err != nil {
		obj.Set("available", false)
		return obj
	}

	obj.Set("available", brewInfo.Available)

	formulae := make([]string, 0, len(brewInfo.Formulae))
	for name := range brewInfo.Formulae {
		formulae = append(formulae, name)
	}
	obj.Set("formulae", formulae)

	casks := make([]string, 0, len(brewInfo.Casks))
	for name := range brewInfo.Casks {
		casks = append(casks, name)
	}
	obj.Set("casks", casks)
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
		obj.Set("exists", fi.Exists)
		obj.Set("hash", fi.Hash)
		obj.Set("mode", int64(fi.Mode))
		obj.Set("size", fi.Size)
		obj.Set("isDir", fi.IsDir)
		obj.Set("isLink", fi.IsLink)
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
		obj.Set("exists", di.Exists)
		obj.Set("mode", int64(di.Mode))
		obj.Set("children", di.Children)
		return obj
	}
}
