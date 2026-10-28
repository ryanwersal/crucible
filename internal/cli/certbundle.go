package cli

import (
	"fmt"
	"github.com/ryanwersal/crucible/internal/certbundle"
	"github.com/spf13/cobra"
)

// newFilterCertBundleCmd supplies a narrow privilege boundary: the child only
// filters the approved bundle, never evaluates JS or executes a whole plan.
func newFilterCertBundleCmd() *cobra.Command {
	return &cobra.Command{
		Use: "_filter-cert-bundle PATH SNAPSHOT FINGERPRINT...", Hidden: true,
		Args: cobra.MinimumNArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			backup, err := certbundle.Apply(cmd.Context(), args[0], args[1], args[2:])
			if err != nil {
				return err
			}
			if backup != "" {
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "Bundle backup: %s\n", backup)
			}
			return err
		},
	}
}
