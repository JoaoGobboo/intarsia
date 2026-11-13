// Package project reads, validates and writes the project files: the source of
// truth for everything the user defines. The web interface and the command
// line both go through it, so they always see the same thing.
package project

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"github.com/goccy/go-yaml"
)

// FormatVersion is the version of the project file format this build reads.
// An incompatible change to the format needs a new version and a migration.
const FormatVersion = 1

const (
	manifestFile  = "project.yaml"
	sourcesDir    = "sources"
	fileExtension = ".yaml"
)

// laterKinds are the item kinds that the format defines but this version does
// not read yet.
var laterKinds = []string{"models", "ontology", "outputs"}

// Manifest is project.yaml.
type Manifest struct {
	Name        string `yaml:"name"`
	Format      int    `yaml:"format"`
	Description string `yaml:"description,omitempty"`
}

// SourceType selects the connector of a source.
type SourceType string

const (
	SourcePostgres SourceType = "postgres"
	SourceExcel    SourceType = "excel"
	SourceCSV      SourceType = "csv"
	SourceParquet  SourceType = "parquet"
	SourcePDF      SourceType = "pdf"
)

// Values lists the accepted source types.
func (SourceType) Values() []string {
	return []string{string(SourcePostgres), string(SourceExcel), string(SourceCSV), string(SourceParquet), string(SourcePDF)}
}

// Mode is how the data of a source is reached.
type Mode string

// ModeAttach queries the source in place, without copying it.
const ModeAttach Mode = "attach"

// Values lists the accepted modes.
func (Mode) Values() []string { return []string{string(ModeAttach)} }

// Source is one file in sources/. Its name is the file name without the
// extension.
type Source struct {
	Name        string     `yaml:"-"`
	Description string     `yaml:"description,omitempty"`
	Type        SourceType `yaml:"type"`
	Mode        Mode       `yaml:"mode"`
	// Writable allows writing to the source. Sources are read-only without it.
	Writable   bool       `yaml:"writable,omitempty"`
	Connection Connection `yaml:"connection"`
	// Tables restricts the source to these tables. Empty means all of them.
	Tables []string `yaml:"tables,omitempty"`
}

// Connection is the part of a source that depends on its type. The
// implementations are *PostgresConnection and *FileConnection.
type Connection interface {
	isConnection()
}

// PostgresConnection connects to a PostgreSQL server.
type PostgresConnection struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port,omitempty"`
	Database string `yaml:"database"`
	User     string `yaml:"user"`
	Password Secret `yaml:"password"`
}

// FileConnection points to a file: Excel, CSV, Parquet or PDF.
type FileConnection struct {
	// Path is absolute or relative to the project directory.
	Path string `yaml:"path"`
}

func (*PostgresConnection) isConnection() {}
func (*FileConnection) isConnection()     {}

// NewConnection returns an empty connection for a source type, or nil when
// the type is unknown.
func NewConnection(t SourceType) Connection {
	switch t {
	case SourcePostgres:
		return &PostgresConnection{}
	case SourceExcel, SourceCSV, SourceParquet, SourcePDF:
		return &FileConnection{}
	default:
		return nil
	}
}

// Project is a project directory, loaded.
type Project struct {
	Dir      string
	Manifest Manifest
	// Sources is sorted by name.
	Sources []Source
}

// itemName is what a file name must look like, because the name becomes an
// identifier in queries (crm.clientes).
var itemName = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

const itemNameRule = "use lowercase letters, digits and underscores, starting with a letter"

// Load reads and validates the project in dir. It reports every problem it
// finds instead of stopping at the first one; the project is usable only when
// the diagnostics contain no error.
func Load(dir string, lookup LookupFunc) (*Project, Diagnostics) {
	p := &Project{Dir: dir}
	var diags Diagnostics

	data, err := os.ReadFile(filepath.Join(dir, manifestFile))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return p, Diagnostics{{File: manifestFile, Line: 1, Column: 1,
			Message: "file not found: this directory is not an Intársia project (create one with `intarsia init`)"}}
	case err != nil:
		return p, Diagnostics{readFailure(manifestFile, err)}
	}
	manifest, found := decodeManifest(manifestFile, data)
	p.Manifest = manifest
	diags = append(diags, found...)

	entries, err := os.ReadDir(filepath.Join(dir, sourcesDir))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		diags = append(diags, readFailure(sourcesDir+"/", err))
	}
	for _, entry := range entries {
		file := path.Join(sourcesDir, entry.Name())
		switch {
		case strings.HasPrefix(entry.Name(), "."):
			continue
		case entry.IsDir() || path.Ext(entry.Name()) != fileExtension:
			diags = append(diags, Diagnostic{Severity: SeverityWarning, File: file,
				Message: "ignored: a source is a file ending in " + fileExtension})
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, sourcesDir, entry.Name()))
		if err != nil {
			diags = append(diags, readFailure(file, err))
			continue
		}
		name := strings.TrimSuffix(entry.Name(), fileExtension)
		source, found := decodeSource(name, file, data, lookup)
		diags = append(diags, found...)
		if found.Errors() == 0 {
			p.Sources = append(p.Sources, source)
		}
	}

	for _, kind := range laterKinds {
		if n := countVisible(filepath.Join(dir, kind)); n > 0 {
			diags = append(diags, Diagnostic{Severity: SeverityWarning, File: kind + "/",
				Message: fmt.Sprintf("ignored: %s are not supported yet (%d found)", kind, n)})
		}
	}
	return p, diags
}

func readFailure(file string, err error) Diagnostic {
	// The *PathError text repeats the absolute path; the cause is enough.
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		err = pathErr.Err
	}
	return Diagnostic{File: file, Line: 1, Column: 1, Message: fmt.Sprintf("cannot read: %v", err)}
}

func countVisible(dir string) int {
	entries, _ := os.ReadDir(dir) // a directory that is missing or unreadable has nothing to ignore
	n := 0
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), ".") {
			n++
		}
	}
	return n
}

// Init creates a new project in dir, creating the directory if needed.
func Init(dir, name string) error {
	manifestPath := filepath.Join(dir, manifestFile)
	if _, err := os.Stat(manifestPath); err == nil {
		return fmt.Errorf("%s already exists: this directory is already a project", manifestPath)
	}
	if err := os.MkdirAll(filepath.Join(dir, sourcesDir), 0o755); err != nil {
		return err
	}
	data, err := marshal(Manifest{Name: name, Format: FormatVersion})
	if err != nil {
		return err
	}
	if err := writeFileAtomic(manifestPath, data); err != nil {
		return err
	}

	ignorePath := filepath.Join(dir, ".gitignore")
	if _, err := os.Stat(ignorePath); errors.Is(err, fs.ErrNotExist) {
		return writeFileAtomic(ignorePath, []byte("# Cache and snapshots, rebuilt from the sources.\ndata/\n"))
	}
	return nil
}

// WriteSource creates or updates sources/<name>.yaml. An existing file keeps
// its comments, key order and formatting: only what changed is rewritten, so a
// source saved without changes stays byte for byte the same.
//
// WriteSource does not validate the source; callers validate before saving.
func WriteSource(dir string, source Source) error {
	// The name becomes a file path, so it is checked even though the rest is not.
	if !itemName.MatchString(source.Name) {
		return fmt.Errorf("invalid source name %q: %s", source.Name, itemNameRule)
	}
	file := filepath.Join(dir, sourcesDir, source.Name+fileExtension)

	original, err := os.ReadFile(file)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	var before any
	if current, ok := decodeSourceLenient(original); ok {
		before = current
	}
	updated, err := rewrite(original, before, source)
	if err != nil {
		return fmt.Errorf("%s: %w", path.Join(sourcesDir, source.Name+fileExtension), err)
	}

	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	return writeFileAtomic(file, updated)
}

// writeFileAtomic replaces the file in one step, so that a crash in the middle
// of a save never leaves a project file half written.
func writeFileAtomic(file string, data []byte) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(file), "."+filepath.Base(file)+".*.tmp")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
		}
	}()

	if _, err = tmp.Write(data); err != nil {
		return err
	}
	if err = tmp.Chmod(0o644); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), file)
}

func marshal(v any) ([]byte, error) {
	return yaml.MarshalWithOptions(v, yaml.IndentSequence(true))
}

// field is one YAML field of a struct.
type field struct {
	name     string
	required bool
	typ      reflect.Type
	// value is set by fieldsOf only.
	value reflect.Value
}

// fieldsOfType lists the YAML fields of a struct type, in declaration order.
// A field is required unless its tag says omitempty; the schema and the
// validation both follow this rule, so they cannot disagree.
func fieldsOfType(t reflect.Type) []field {
	return collectFields(t, reflect.Value{})
}

// fieldsOf is fieldsOfType for a struct value, with each field's value.
func fieldsOf(v reflect.Value) []field {
	return collectFields(v.Type(), v)
}

func collectFields(t reflect.Type, v reflect.Value) []field {
	var fields []field
	for i := range t.NumField() {
		sf := t.Field(i)
		name, options, _ := strings.Cut(sf.Tag.Get("yaml"), ",")
		if name == "-" {
			continue
		}
		f := field{name: name, required: !strings.Contains(options, "omitempty"), typ: sf.Type}
		if v.IsValid() {
			f.value = v.Field(i)
		}
		fields = append(fields, f)
	}
	return fields
}
