package resource

import (
	"context"
	"errors"
	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/script/decl"
	"io"
	"testing"
)

type keychainStub struct {
	available bool
	err       error
	stored    bool
}

func (s *keychainStub) Available(context.Context, string) (bool, error) { return s.available, s.err }
func (s *keychainStub) Store(context.Context, string, io.Reader, io.Writer, io.Writer) error {
	s.stored = true
	return s.err
}
func TestSSHKeychainPlan(t *testing.T) {
	for _, name := range []string{"available", "missing", "failed"} {
		t.Run(name, func(t *testing.T) {
			client := &keychainStub{available: name == "available"}
			if name == "failed" {
				client.err = errors.New("key unreadable")
			}
			result, err := (SSHKeychainHandler{Client: client}).Plan(t.Context(), fact.NewStore(), Env{}, decl.Declaration{Path: "/key"})
			if client.stored {
				t.Fatal("planning stored a passphrase")
			}
			if name == "failed" {
				if !errors.Is(err, client.err) {
					t.Fatalf("error = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if name == "available" {
				if len(result.Actions) != 0 {
					t.Fatal("planned unnecessary enrollment")
				}
				return
			}
			if len(result.Actions) != 1 || !result.Actions[0].NeedsTerminal || result.Actions[0].NeedsSudo || result.Actions[0].Type != action.StoreSSHKeychain {
				t.Fatalf("unexpected plan: %+v", result)
			}
		})
	}
}
