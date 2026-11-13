package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newRootCmd(version string) *cobra.Command {
	root := &cobra.Command{
		Use:   "intarsia",
		Short: "Unify databases, spreadsheets and APIs on a single machine",
		// Errors are printed once, by main; usage is noise for runtime failures.
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.AddCommand(newVersionCmd(version), newServeCmd(), newInitCmd(), newValidateCmd())
	return root
}

func newVersionCmd(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "intarsia %s\n", version)
			return err
		},
	}
}
