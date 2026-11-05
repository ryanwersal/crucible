package fact

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"golang.org/x/sync/errgroup"
)

type countCollector struct {
	calls atomic.Int32
	val   string
}

func (c *countCollector) Collect(_ context.Context) (string, error) {
	c.calls.Add(1)
	return c.val, nil
}

type errCollector struct{}

func (e errCollector) Collect(_ context.Context) (string, error) {
	return "", errors.New("boom")
}

func TestGet_CachesResult(t *testing.T) {
	t.Parallel()
	store := NewStore()
	c := &countCollector{val: "hello"}

	v1, err := Get(context.Background(), store, "test", c)
	if err != nil {
		t.Fatal(err)
	}
	v2, err := Get(context.Background(), store, "test", c)
	if err != nil {
		t.Fatal(err)
	}

	if v1 != "hello" || v2 != "hello" {
		t.Fatalf("expected hello, got %q and %q", v1, v2)
	}
	if c.calls.Load() != 1 {
		t.Fatalf("expected 1 collect call, got %d", c.calls.Load())
	}
}

func TestGet_DifferentKeys(t *testing.T) {
	t.Parallel()
	store := NewStore()
	c1 := &countCollector{val: "a"}
	c2 := &countCollector{val: "b"}

	v1, _ := Get(context.Background(), store, "k1", c1)
	v2, _ := Get(context.Background(), store, "k2", c2)

	if v1 != "a" || v2 != "b" {
		t.Fatalf("got %q and %q", v1, v2)
	}
}

func TestGet_PropagatesError(t *testing.T) {
	t.Parallel()
	store := NewStore()
	_, err := Get(context.Background(), store, "err", errCollector{})
	if err == nil {
		t.Fatal("expected error")
	}
}

// fatalCollector fails the test if invoked; it guards against the seeded-entry
// short-circuit regressing into a normal collector call.
type fatalCollector struct {
	t *testing.T
}

func (f fatalCollector) Collect(_ context.Context) (string, error) {
	f.t.Fatal("collector should not be called for a seeded entry")
	return "", nil
}

func TestSet_ShortCircuitsCollector(t *testing.T) {
	t.Parallel()
	store := NewStore()
	Set(store, "stub", "seeded value")

	v, err := Get(context.Background(), store, "stub", fatalCollector{t})
	if err != nil {
		t.Fatal(err)
	}
	if v != "seeded value" {
		t.Fatalf("got %q, want %q", v, "seeded value")
	}
}

func TestSet_RepeatedGetsReturnSameValue(t *testing.T) {
	t.Parallel()
	store := NewStore()
	Set(store, "stub", "v")

	for i := range 3 {
		v, err := Get(context.Background(), store, "stub", fatalCollector{t})
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		if v != "v" {
			t.Fatalf("call %d: got %q, want %q", i, v, "v")
		}
	}
}

type intCollector struct{}

func (intCollector) Collect(_ context.Context) (int, error) { return 0, nil }

func TestSet_TypeMismatchPropagates(t *testing.T) {
	t.Parallel()
	store := NewStore()
	Set(store, "stub", "string value")

	// Get with a different T should report cached type mismatch rather
	// than invoking the collector or returning a zero value silently.
	_, err := Get(context.Background(), store, "stub", intCollector{})
	if err == nil {
		t.Fatal("expected type mismatch error")
	}
}

func TestSetCollectorDefersAndSharesCollection(t *testing.T) {
	t.Parallel()
	store := NewStore()
	configured := &countCollector{val: "configured"}
	fallback := &countCollector{val: "fallback"}
	SetCollector(store, "test", configured)
	if configured.calls.Load() != 0 {
		t.Fatal("registration collected facts")
	}
	var group errgroup.Group
	group.SetLimit(8)
	for range 16 {
		group.Go(func() error {
			value, err := Get(context.Background(), store, "test", fallback)
			if err != nil {
				return err
			}
			if value != "configured" {
				return errors.New("configured collector ignored")
			}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		t.Fatal(err)
	}
	if configured.calls.Load() != 1 || fallback.calls.Load() != 0 {
		t.Fatal("collection was not shared")
	}
}

func TestSetCollectorPreservesExistingEntries(t *testing.T) {
	t.Parallel()
	for _, seeded := range []bool{true, false} {
		store := NewStore()
		if seeded {
			Set(store, "test", "existing")
		} else {
			if _, err := Get(context.Background(), store, "test", &countCollector{val: "existing"}); err != nil {
				t.Fatal(err)
			}
		}
		SetCollector(store, "test", fatalCollector{t})
		value, err := Get(context.Background(), store, "test", fatalCollector{t})
		if err != nil || value != "existing" {
			t.Fatalf("existing entry lost: %q %v", value, err)
		}
	}
}

func TestSetCollectorCachesErrors(t *testing.T) {
	t.Parallel()
	store := NewStore()
	SetCollector(store, "test", errCollector{})
	for range 2 {
		if _, err := Get(context.Background(), store, "test", fatalCollector{t}); err == nil {
			t.Fatal("collection failure was hidden")
		}
	}
}
