package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/JoaoGobboo/intarsia/internal/project"
)

func newInitCmd() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "init [directory]",
		Short: "Create a new project",
		Long:  "Create a new project in the directory, or in the current one when none is given.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := projectDir(args)
			if name == "" {
				absolute, err := filepath.Abs(dir)
				if err != nil {
					return err
				}
				name = filepath.Base(absolute)
			}
			if err := project.Init(dir, name); err != nil {
				return err
			}
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "Created project %q in %s\n", name, dir)
			return err
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "project name (default: the name of the directory)")
	return cmd
}

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate [directory]",
		Short: "Check the project files",
		Long: "Check the project files in the directory, or in the current one when none is given.\n" +
			"Every problem is reported with its file, line and field.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			p, diagnostics := project.Load(projectDir(args), os.LookupEnv)
			for _, d := range diagnostics {
				if _, err := fmt.Fprintln(out, d); err != nil {
					return err
				}
			}
			if n := diagnostics.Errors(); n > 0 {
				return fmt.Errorf("the project is not valid: %d error(s)", n)
			}
			_, err := fmt.Fprintf(out, "Project %q is valid: %d source(s)\n", p.Manifest.Name, len(p.Sources))
			return err
		},
	}
}

func projectDir(args []string) string {
	if len(args) == 0 {
		return "."
	}
	return args[0]
}
