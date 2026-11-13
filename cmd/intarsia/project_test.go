package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// execute runs the command line and returns what it printed.
func execute(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := newRootCmd("dev")
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestInitThenValidate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "vendas")

	out, err := execute(t, "init", dir)
	if err != nil || !strings.Contains(out, `Created project "vendas"`) {
		t.Fatalf("init = %q, %v", out, err)
	}
	out, err = execute(t, "validate", dir)
	if err != nil || out != "Project \"vendas\" is valid: 0 source(s)\n" {
		t.Fatalf("validate = %q, %v", out, err)
	}
	if _, err := execute(t, "init", dir); err == nil {
		t.Error("second init error = nil, want an error")
	}
}

func TestInitUsesTheGivenName(t *testing.T) {
	dir := t.TempDir()
	if _, err := execute(t, "init", dir, "--name", "Vendas 2026"); err != nil {
		t.Fatal(err)
	}
	out, err := execute(t, "validate", dir)
	if err != nil || !strings.Contains(out, `Project "Vendas 2026" is valid`) {
		t.Errorf("validate = %q, %v", out, err)
	}
}

func TestValidateReportsFileLineAndField(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "vendas")
	if _, err := execute(t, "init", dir); err != nil {
		t.Fatal(err)
	}
	source := "type: csv\nmode: attach\nconnection:\n  path: a.csv\n  sheet: Plan1\n"
	if err := os.WriteFile(filepath.Join(dir, "sources", "metas.yaml"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := execute(t, "validate", dir)
	if err == nil || err.Error() != "the project is not valid: 1 error(s)" {
		t.Errorf("validate error = %v, want a failure counting 1 error", err)
	}
	if want := "sources/metas.yaml:5:3: error: connection.sheet: unknown field\n"; out != want {
		t.Errorf("validate printed %q, want %q", out, want)
	}
}
