package script

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/console"
	"github.com/dop251/goja_nodejs/require"

	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/script/modules"
)

// Runtime wraps a goja JavaScript VM with crucible's native modules and
// declaration collection. A Runtime is single-threaded and created per
// Plan() invocation.
type Runtime struct {
	vm           *goja.Runtime
	registry     *require.Registry
	logger       *slog.Logger
	sourceDir    string
	targetDir    string
	declarations []Declaration
}

// NewRuntime creates a script runtime with native modules registered.
// The provided store should already have expensive facts pre-collected.
func NewRuntime(ctx context.Context, logger *slog.Logger, sourceDir, targetDir string, store *fact.Store) *Runtime {
	vm := goja.New()
	vm.SetFieldNameMapper(goja.UncapFieldNameMapper())

	r := &Runtime{
		vm:        vm,
		logger:    logger,
		sourceDir: sourceDir,
		targetDir: targetDir,
	}

	// Set up the require registry with source dir as the base for relative requires
	r.registry = require.NewRegistry(
		require.WithGlobalFolders(sourceDir),
	)
	r.registry.Enable(vm)

	// Enable console (wired to stdout via goja_nodejs)
	console.Enable(vm)

	// Register native modules
	r.registry.RegisterNativeModule("crucible", func(runtime *goja.Runtime, module *goja.Object) {
		mod := modules.NewCrucibleModule(runtime, logger, targetDir, &r.declarations)
		module.Set("exports", mod.Export())
	})

	r.registry.RegisterNativeModule("crucible/facts", func(runtime *goja.Runtime, module *goja.Object) {
		mod := modules.NewFactsModule(runtime, ctx, store)
		module.Set("exports", mod.Export())
	})

	return r
}

// Execute runs a script and returns the declarations it produced.
// The context is used for interrupt support — cancelling ctx will
// interrupt the VM.
func (r *Runtime) Execute(ctx context.Context, file string, source []byte) ([]Declaration, error) {
	// Start interrupt goroutine
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			r.vm.Interrupt("context cancelled")
		case <-done:
		}
	}()

	_, err := r.vm.RunScript(file, string(source))
	if err != nil {
		return nil, wrapGojaError(err, file)
	}

	return r.declarations, nil
}

// Declarations returns the accumulated declarations.
func (r *Runtime) Declarations() []Declaration {
	return r.declarations
}

// ResolveContent resolves source file references and templates in declarations.
// This must be called after Execute and before converting to actions.
func (r *Runtime) ResolveContent(ctx context.Context, store *fact.Store) error {
	for i := range r.declarations {
		decl := &r.declarations[i]
		if decl.Type != DeclFile {
			continue
		}

		if err := r.resolveFileContent(ctx, store, decl); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runtime) resolveFileContent(_ context.Context, _ *fact.Store, decl *Declaration) error {
	// Source file reference: read file from source dir
	if decl.SourceFile != "" {
		path := filepath.Join(r.sourceDir, decl.SourceFile)
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read source file %s: %w", decl.SourceFile, err)
		}
		decl.Content = content
		return nil
	}

	// Template file reference: read and render Go template
	if decl.TemplateFile != "" {
		path := filepath.Join(r.sourceDir, decl.TemplateFile)
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read template %s: %w", decl.TemplateFile, err)
		}

		rendered, err := renderTemplate(decl.TemplateFile, string(content), decl.TemplateData)
		if err != nil {
			return fmt.Errorf("render template %s: %w", decl.TemplateFile, err)
		}
		decl.Content = rendered
		return nil
	}

	return nil
}
