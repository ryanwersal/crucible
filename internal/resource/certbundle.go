package resource

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ryanwersal/crucible/internal/action"
	"github.com/ryanwersal/crucible/internal/certbundle"
	"github.com/ryanwersal/crucible/internal/fact"
	"github.com/ryanwersal/crucible/internal/script/decl"
)

// CertBundleHandler plans exact-certificate removals from a PEM bundle.
type CertBundleHandler struct{}

func (CertBundleHandler) DeclType() decl.Type { return decl.CertBundle }
func (CertBundleHandler) DeclName() string    { return "CertBundle" }
func (CertBundleHandler) Plan(ctx context.Context, store *fact.Store, env Env, d decl.Declaration) (PlanOutput, error) {
	count := 0
	for _, other := range env.Declarations {
		if other.Path == d.Path {
			count++
		}
	}
	if count > 1 {
		return PlanOutput{}, fmt.Errorf("certificate bundle path has multiple declarations: %s", d.Path)
	}
	snapshot, err := fact.Get(ctx, store, "certBundle:"+d.Path, fact.CertBundleCollector{Path: d.Path})
	if err != nil {
		return PlanOutput{}, err
	}
	_, removed, err := certbundle.Filter(snapshot.Content, d.CertAbsent)
	if err != nil {
		return PlanOutput{}, err
	}
	if len(removed) == 0 {
		return PlanOutput{Observations: []action.Observation{{Description: d.Path + " (denied certificates absent)"}}}, nil
	}
	labels := make([]string, 0, len(removed))
	for _, cert := range removed {
		labels = append(labels, cert.Subject+" [SHA-256 "+cert.Fingerprint+"]")
	}
	return PlanOutput{Actions: []action.Action{{
		Type: action.FilterCertBundle, Path: d.Path, NeedsSudo: d.CertSudo,
		CertAbsent: d.CertAbsent, CertSnapshot: snapshot.Token,
		Description: "remove certificates from " + d.Path + ": " + strings.Join(labels, "; "),
		Note:        "A versioned backup is retained. Removing trust anchors can break otherwise valid sites. Only this PEM bundle is covered.",
	}}}, nil
}

// FilterCertBundleExecutor revalidates and applies the approved removal plan.
type FilterCertBundleExecutor struct{}

func (FilterCertBundleExecutor) ActionType() action.Type { return action.FilterCertBundle }
func (FilterCertBundleExecutor) ActionName() string      { return "FilterCertBundle" }
func (FilterCertBundleExecutor) Execute(ctx context.Context, a action.Action, stdin io.Reader, stdout, stderr io.Writer) error {
	if a.NeedsSudo {
		executable, err := os.Executable()
		if err != nil {
			return err
		}
		args := append([]string{"_filter-cert-bundle", a.Path, a.CertSnapshot}, a.CertAbsent...)
		return runCmd(ctx, a, stdin, stdout, stderr, executable, args...)
	}
	backup, err := certbundle.Apply(ctx, a.Path, a.CertSnapshot, a.CertAbsent)
	if err != nil {
		return err
	}
	if backup != "" && stdout != nil {
		_, err = fmt.Fprintf(stdout, "Bundle backup: %s\n", backup)
	}
	return err
}
