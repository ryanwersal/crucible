package fact

import (
	"context"
	"os"
	"runtime"
)

// OSInfo holds basic OS-level facts.
type OSInfo struct {
	OS       string // runtime.GOOS
	Arch     string // runtime.GOARCH
	Hostname string
}

// OSCollector collects OS-level facts.
type OSCollector struct{}

func (o OSCollector) Collect(_ context.Context) (*OSInfo, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	return &OSInfo{
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		Hostname: hostname,
	}, nil
}
