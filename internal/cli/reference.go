package cli

import (
	"fmt"

	"github.com/ryanwersal/crucible/internal/reference"
	"github.com/spf13/cobra"
)

func newReferenceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reference",
		Short: "Print comprehensive reference documentation",
		Long:  "Dumps a structured plain-text reference of the full crucible API surface to stdout. Designed for LLM consumption.",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Print(reference.Build(cmd.Root()))
		},
	}
}
