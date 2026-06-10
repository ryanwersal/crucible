package fact

import (
	"context"
	"fmt"
	"sync"
)

// Collector gathers a specific type of system state.
// T is the structured result — no type assertions needed at call sites.
type Collector[T any] interface {
	Collect(ctx context.Context) (T, error)
}

// entry holds a in-flight or completed collection for a single key.
// seeded entries (via Set) skip the collector path entirely.
type entry struct {
	once   sync.Once
	val    any
	err    error
	seeded bool // true when val came from Set; Get must not invoke the collector
}

// Store caches collected facts for the duration of a plan phase.
type Store struct {
	mu      sync.Mutex
	entries map[string]*entry
}

// NewStore creates an empty fact store.
func NewStore() *Store {
	return &Store{entries: make(map[string]*entry)}
}

// Set seeds a pre-computed fact for the given key. A subsequent Get for the
// same key returns this value without invoking the collector. Useful for
// tests that want to inject fixture facts and skip shelling out to real
// system tools. Set must be called before Get for the same key.
func Set[T any](s *Store, key string, val T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[key] = &entry{val: val, seeded: true}
}

// Get retrieves a cached fact or collects it exactly once per key.
// Concurrent calls for the same key block until the first collection completes.
func Get[T any](ctx context.Context, s *Store, key string, c Collector[T]) (T, error) {
	s.mu.Lock()
	e, ok := s.entries[key]
	if !ok {
		e = &entry{}
		s.entries[key] = e
	}
	s.mu.Unlock()

	if !e.seeded {
		e.once.Do(func() {
			e.val, e.err = c.Collect(ctx)
		})
	}

	if e.err != nil {
		var zero T
		return zero, fmt.Errorf("fact %q: %w", key, e.err)
	}

	t, ok := e.val.(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("fact %q: cached type mismatch", key)
	}
	return t, nil
}
