package resource

import (
	"context"
	"fmt"
	"io"

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
}

// RegisterBatchHandler registers a batch handler.
func (r *Registry) RegisterBatchHandler(b BatchHandler) {
	r.batchers[b.DeclType()] = b
}

// RegisterExecutor registers an action executor.
func (r *Registry) RegisterExecutor(e ActionExecutor) {
	r.executors[e.ActionType()] = e
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
func (r *Registry) Execute(ctx context.Context, a action.Action, stdout, stderr io.Writer) error {
	e, ok := r.executors[a.Type]
	if !ok {
		return fmt.Errorf("no executor registered for action type %v", a.Type)
	}
	return e.Execute(ctx, a, stdout, stderr)
}

// Validate checks that every known declaration and action type has a
// registered handler/executor. Returns an error listing any gaps.
func (r *Registry) Validate() error {
	var missing []string
	for _, t := range decl.AllTypes() {
		if _, ok := r.handlers[t]; !ok {
			if _, ok := r.batchers[t]; !ok {
				missing = append(missing, fmt.Sprintf("decl type %v has no handler or batch handler", t))
			}
		}
	}
	for _, t := range action.AllTypes() {
		if _, ok := r.executors[t]; !ok {
			missing = append(missing, fmt.Sprintf("action type %v has no executor", t))
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("registry validation failed:\n  %s", fmt.Sprintf("%s", joinLines(missing)))
	}
	return nil
}

// DefaultRegistry returns a registry with all built-in handlers and executors.
func DefaultRegistry() *Registry {
	r := NewRegistry()

	// Per-declaration handlers
	r.RegisterHandler(FileHandler{})
	r.RegisterHandler(DirHandler{})
	r.RegisterHandler(SymlinkHandler{})
	r.RegisterHandler(DefaultsHandler{})
	r.RegisterHandler(DockHandler{})
	r.RegisterHandler(GitRepoHandler{})
	r.RegisterHandler(ShellHandler{})

	// Batch handlers
	r.RegisterBatchHandler(PackageHandler{})
	r.RegisterBatchHandler(FontHandler{})
	r.RegisterBatchHandler(MiseToolHandler{})

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

	return r
}

func joinLines(ss []string) string {
	result := ss[0]
	for _, s := range ss[1:] {
		result += "\n  " + s
	}
	return result
}
