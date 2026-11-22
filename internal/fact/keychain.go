package fact

import (
	"context"
	"github.com/ryanwersal/crucible/internal/keychain"
)

type SSHKeychainInfo struct{ Available bool }
type SSHKeychainCollector struct {
	Path   string
	Client keychain.SSHKeys
}

func (c SSHKeychainCollector) Collect(ctx context.Context) (*SSHKeychainInfo, error) {
	client := c.Client
	if client == nil {
		client = keychain.SSHClient{}
	}
	available, err := client.Available(ctx, c.Path)
	if err != nil {
		return nil, err
	}
	return &SSHKeychainInfo{Available: available}, nil
}
