package fact

import (
	"context"
	"github.com/ryanwersal/crucible/internal/certbundle"
)

// CertBundleCollector reads a bundle snapshot for fingerprint-based planning.
type CertBundleCollector struct{ Path string }

func (c CertBundleCollector) Collect(ctx context.Context) (*certbundle.Snapshot, error) {
	snapshot, err := certbundle.Read(ctx, c.Path)
	if err != nil {
		return nil, err
	}
	if _, err := certbundle.Parse(snapshot.Content); err != nil {
		return nil, err
	}
	return snapshot, nil
}
