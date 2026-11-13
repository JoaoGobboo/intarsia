package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/JoaoGobboo/intarsia/demo"
)

func newDemoCmd() *cobra.Command {
	var dir string

	cmd := &cobra.Command{
		Use:   "demo",
		Short: "Run the fictional company used for tests and demonstrations",
		Long: "Run the fictional company: synthetic data, always the same, spread over a\n" +
			"Postgres database, a spreadsheet and a PDF. Needs Docker.",
	}
	cmd.PersistentFlags().StringVar(&dir, "dir", "demo", "the demo directory of the repository")

	cmd.AddCommand(&cobra.Command{
		Use:   "up",
		Short: "Generate the data and start the database",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := checkDemoDir(dir); err != nil {
				return err
			}
			if err := demo.Write(dir); err != nil {
				return fmt.Errorf("generate the demo data: %w", err)
			}
			// --wait returns once the database accepts connections, with the data loaded.
			if err := compose(cmd, dir, "up", "--detach", "--wait"); err != nil {
				return err
			}
			return printDemoHelp(cmd.OutOrStdout(), dir)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "down",
		Short: "Stop the database and discard its data",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := checkDemoDir(dir); err != nil {
				return err
			}
			return compose(cmd, dir, "down", "--volumes")
		},
	})
	return cmd
}

func checkDemoDir(dir string) error {
	if _, err := os.Stat(filepath.Join(dir, demo.ComposeFile)); err != nil {
		return fmt.Errorf("%s not found in %q: run from the root of the repository or pass --dir", demo.ComposeFile, dir)
	}
	return nil
}

// compose runs `docker compose` on the demo, showing its output.
func compose(cmd *cobra.Command, dir string, args ...string) error {
	args = append([]string{"compose", "--file", filepath.Join(dir, demo.ComposeFile)}, args...)
	docker := exec.CommandContext(cmd.Context(), "docker", args...)
	docker.Stdout = cmd.ErrOrStderr() // progress, not the result of the command
	docker.Stderr = cmd.ErrOrStderr()

	err := docker.Run()
	switch {
	case errors.Is(err, exec.ErrNotFound):
		return errors.New("docker not found: the demo needs Docker with the compose plugin installed")
	case errors.Is(cmd.Context().Err(), context.Canceled):
		return errors.New("interrupted")
	case err != nil:
		return fmt.Errorf("docker compose failed (is Docker running?): %w", err)
	}
	return nil
}

func printDemoHelp(out io.Writer, dir string) error {
	projectDir := filepath.Join(dir, "project")
	_, err := fmt.Fprintf(out, `The demo is running.

  CRM (Postgres)   127.0.0.1:54320, database crm
  Project          %[1]s

The project reads the database credentials from the environment. Set them:

  PowerShell   $env:CRM_USER = "%[2]s"; $env:CRM_PASSWORD = "%[3]s"
  bash, zsh    export CRM_USER=%[2]s CRM_PASSWORD=%[3]s

Then check the project:

  intarsia validate %[1]s

Stop the demo with: intarsia demo down
`, projectDir, demo.CRMUser, demo.CRMPassword)
	return err
}
