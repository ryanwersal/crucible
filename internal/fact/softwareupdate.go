package fact

import (
	"context"

	"github.com/ryanwersal/crucible/internal/softwareupdate"
)

type SoftwareUpdateInfo struct {
	Receipts  map[string]bool
	Updates   []softwareupdate.Update
	CLTUsable bool
}

type SoftwareUpdateCollector struct {
	Client   *softwareupdate.Client
	Requests []softwareupdate.Request
}

func (c SoftwareUpdateCollector) Collect(ctx context.Context) (*SoftwareUpdateInfo, error) {
	client := c.Client
	if client == nil {
		client = softwareupdate.NewClient()
	}
	receipts, err := client.Receipts(ctx)
	if err != nil {
		return nil, err
	}
	info := &SoftwareUpdateInfo{Receipts: receipts, CLTUsable: client.CLTUsable()}
	scan, discover := false, false
	for _, request := range c.Requests {
		installed := receipts[request.Receipt] && (!request.CommandLineTools || info.CLTUsable)
		if request.Latest || !installed {
			scan = true
			discover = discover || (request.CommandLineTools && !installed)
		}
	}
	if scan {
		info.Updates, err = client.Catalog(ctx, discover)
		if err != nil {
			return nil, err
		}
	}
	return info, nil
}
