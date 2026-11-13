package demo

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
)

// ComposeFile is the Docker Compose file of the demonstration, inside the
// demo directory.
const ComposeFile = "docker-compose.yml"

// Credentials of the demonstration database. They are not secrets: the
// database holds only synthetic data and listens only on this machine. The
// project still reads them from the environment, like any project must.
const (
	CRMUser     = "crm"
	CRMPassword = "demo"
)

// File is one generated file of the demonstration.
type File struct {
	// Path is relative to the demo directory, with forward slashes.
	Path  string
	write func(io.Writer, Company) error
}

// Files lists what Write generates. Every path is ignored by Git.
func Files() []File {
	return []File{
		{"generated/crm.sql", writeCRM},
		{"project/files/metas.xlsx", writeTargets},
		{"project/files/comissoes.pdf", writeCommissions},
	}
}

// Contents returns the bytes of the file.
func (f File) Contents(c Company) ([]byte, error) {
	var b bytes.Buffer
	if err := f.write(&b, c); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// Write generates the data files of the fictional company inside dir, the
// demo directory. Running it again produces exactly the same files.
func Write(dir string) error {
	company := Generate()
	for _, file := range Files() {
		data, err := file.Contents(company)
		if err != nil {
			return err
		}
		target := filepath.Join(dir, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}
