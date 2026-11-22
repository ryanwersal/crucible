package resource

import (
	"context"
	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/keychain"
	"github.com/ryanwersal/crucible/internal/script/decl"
	"io"
)

type SSHKeychainHandler struct{ Client keychain.SSHKeys }

func (SSHKeychainHandler) DeclType() decl.Type { return decl.SSHKeychain }
func (SSHKeychainHandler) DeclName() string    { return "SSHKeychain" }
func (h SSHKeychainHandler) Plan(ctx context.Context, store *fact.Store, _ Env, d decl.Declaration) (PlanOutput, error) {
	info, err := fact.Get(ctx, store, "ssh-keychain:"+d.Path, fact.SSHKeychainCollector{Path: d.Path, Client: h.Client})
	if err != nil {
		return PlanOutput{}, err
	}
	if info.Available {
		return PlanOutput{Observations: []action.Observation{{Description: d.Path + " (SSH key unlocks without a prompt)"}}}, nil
	}
	return PlanOutput{Actions: []action.Action{{Type: action.StoreSSHKeychain, Path: d.Path, NeedsTerminal: true, Description: "store SSH passphrase in Keychain: " + d.Path}}}, nil
}

type StoreSSHKeychainExecutor struct{ Client keychain.SSHKeys }

func (StoreSSHKeychainExecutor) ActionType() action.Type { return action.StoreSSHKeychain }
func (StoreSSHKeychainExecutor) ActionName() string      { return "StoreSSHKeychain" }
func (e StoreSSHKeychainExecutor) Execute(ctx context.Context, a action.Action, stdin io.Reader, stdout, stderr io.Writer) error {
	client := e.Client
	if client == nil {
		client = keychain.SSHClient{}
	}
	return client.Store(ctx, a.Path, stdin, stdout, stderr)
}
