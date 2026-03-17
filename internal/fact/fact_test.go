package fact

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
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
