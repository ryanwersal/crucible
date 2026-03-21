package resource

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/script/decl"
)

// Registry maps declaration types to handlers and action types to executors.
type Registry struct {
	handlers  map[decl.Type]Handler
	batchers  map[decl.Type]BatchHandler
	executors map[action.Type]ActionExecutor
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		handlers:  make(map[decl.Type]Handler),
		batchers:  make(map[decl.Type]BatchHandler),
		executors: make(map[action.Type]ActionExecutor),
	}
}

// RegisterHandler registers a per-declaration handler.
func (r *Registry) RegisterHandler(h Handler) {
	r.handlers[h.DeclType()] = h
	decl.RegisterName(h.DeclType(), h.DeclName())
}

// RegisterBatchHandler registers a batch handler.
func (r *Registry) RegisterBatchHandler(b BatchHandler) {
	r.batchers[b.DeclType()] = b
	decl.RegisterName(b.DeclType(), b.DeclName())
}

// RegisterExecutor registers an action executor.
func (r *Registry) RegisterExecutor(e ActionExecutor) {
	r.executors[e.ActionType()] = e
	action.RegisterName(e.ActionType(), e.ActionName())
}

// IsBatched reports whether the given declaration type uses batch planning.
func (r *Registry) IsBatched(t decl.Type) bool {
	_, ok := r.batchers[t]
	return ok
}

// PlanOne dispatches a single declaration to its handler.
func (r *Registry) PlanOne(ctx context.Context, store *fact.Store, env Env, d decl.Declaration) (PlanOutput, error) {
	h, ok := r.handlers[d.Type]
	if !ok {
		return PlanOutput{}, fmt.Errorf("no handler registered for declaration type %v", d.Type)
	}
	return h.Plan(ctx, store, env, d)
}

// PlanBatch dispatches a slice of declarations to the batch handler for the given type.
func (r *Registry) PlanBatch(ctx context.Context, store *fact.Store, env Env, t decl.Type, decls []decl.Declaration) (PlanOutput, error) {
	b, ok := r.batchers[t]
	if !ok {
		return PlanOutput{}, fmt.Errorf("no batch handler registered for declaration type %v", t)
	}
	return b.PlanBatch(ctx, store, env, decls)
}

// Execute dispatches a single action to its executor.
func (r *Registry) Execute(ctx context.Context, a action.Action, stdin io.Reader, stdout, stderr io.Writer) error {
	e, ok := r.executors[a.Type]
	if !ok {
		return fmt.Errorf("no executor registered for action type %v", a.Type)
	}
	return e.Execute(ctx, a, stdin, stdout, stderr)
}

// Validate checks internal consistency of the registry. It verifies that:
//   - every registered decl type has exactly one handler (not both handler and batcher)
//   - every registered action type has a name (guaranteed by construction, but defensive)
//   - every registered decl type has a name (same)
func (r *Registry) Validate() error {
	var missing []string

	// Check for decl types registered as both handler and batcher.
	for t := range r.handlers {
		if _, ok := r.batchers[t]; ok {
			missing = append(missing, fmt.Sprintf("decl type %v registered as both handler and batch handler", t))
		}
	}

	// Verify every registered executor's action type has a name.
	for t := range r.executors {
		if t.String() == fmt.Sprintf("action(%d)", t) {
			missing = append(missing, fmt.Sprintf("action type %d has no registered name", t))
		}
	}

	// Verify every registered handler's decl type has a name.
	for t := range r.handlers {
		if t.String() == fmt.Sprintf("decl(%d)", t) {
			missing = append(missing, fmt.Sprintf("decl type %d has no registered name", t))
		}
	}
	for t := range r.batchers {
		if t.String() == fmt.Sprintf("decl(%d)", t) {
			missing = append(missing, fmt.Sprintf("decl type %d has no registered name", t))
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("registry validation failed:\n  %s", joinLines(missing))
	}
	return nil
}

// AllActionTypes returns every action type registered in the registry.
func (r *Registry) AllActionTypes() []action.Type {
	return action.AllTypes()
}

// AllDeclTypes returns every declaration type registered in the registry.
func (r *Registry) AllDeclTypes() []decl.Type {
	return decl.AllTypes()
}

var (
	defaultRegistry     *Registry
	defaultRegistryOnce sync.Once
)

// DefaultRegistry returns a registry with all built-in handlers and executors.
// It is safe to call concurrently; the registry is initialized once.
func DefaultRegistry() *Registry {
	defaultRegistryOnce.Do(func() {
		defaultRegistry = newDefaultRegistry()
	})
	return defaultRegistry
}

func newDefaultRegistry() *Registry {
	r := NewRegistry()

	// Per-declaration handlers
	r.RegisterHandler(FileHandler{})
	r.RegisterHandler(DirHandler{})
	r.RegisterHandler(SymlinkHandler{})
	r.RegisterHandler(DefaultsHandler{})
	r.RegisterHandler(DockHandler{})
	r.RegisterHandler(GitRepoHandler{})
	r.RegisterHandler(ShellHandler{})
	r.RegisterHandler(KeyRemapHandler{})

	// Batch handlers
	r.RegisterBatchHandler(PackageHandler{})
	r.RegisterBatchHandler(FontHandler{})
	r.RegisterBatchHandler(MiseToolHandler{})
	r.RegisterBatchHandler(MasHandler{})

	// Action executors
	r.RegisterExecutor(WriteFileExecutor{})
	r.RegisterExecutor(CreateDirExecutor{})
	r.RegisterExecutor(CreateSymlinkExecutor{})
	r.RegisterExecutor(SetPermissionsExecutor{})
	r.RegisterExecutor(DeletePathExecutor{})
	r.RegisterExecutor(InstallPackageExecutor{})
	r.RegisterExecutor(UninstallPackageExecutor{})
	r.RegisterExecutor(SetDefaultsExecutor{})
	r.RegisterExecutor(DeleteDefaultsExecutor{})
	r.RegisterExecutor(SetDockExecutor{})
	r.RegisterExecutor(CloneRepoExecutor{})
	r.RegisterExecutor(PullRepoExecutor{})
	r.RegisterExecutor(InstallFontExecutor{})
	r.RegisterExecutor(InstallMiseToolExecutor{})
	r.RegisterExecutor(UninstallMiseToolExecutor{})
	r.RegisterExecutor(SetShellExecutor{})
	r.RegisterExecutor(InstallMasAppExecutor{})
	r.RegisterExecutor(SetKeyRemapExecutor{})
	r.RegisterExecutor(RemoveKeyRemapExecutor{})

	return r
}

func joinLines(ss []string) string {
	var b strings.Builder
	b.WriteString(ss[0])
	for _, s := range ss[1:] {
		b.WriteString("\n  ")
		b.WriteString(s)
	}
	return b.String()
}
