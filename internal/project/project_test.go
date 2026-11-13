package project

import (
	"bytes"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the reference files in testdata with the current output")

// checkReference compares got with a reference file. The expected output lives
// next to its input, so a change in behaviour shows up in code review.
func checkReference(t *testing.T, file string, got []byte) {
	t.Helper()
	if *update {
		if err := os.WriteFile(file, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("%v (run `go test ./internal/project -update` to create it)", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s does not match.\n--- got ---\n%s--- want ---\n%s", file, got, want)
	}
}

func testEnv(name string) (string, bool) {
	value, ok := map[string]string{
		"CRM_USER":     "app",
		"CRM_PASSWORD": "s3cr3t-value",
		"DW_HOST":      "dw.interno",
	}[name]
	return value, ok
}

func subdirectories(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	return names
}

func TestLoad(t *testing.T) {
	const root = "testdata/load"
	for _, name := range subdirectories(t, root) {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(root, name)
			_, diags := Load(dir, testEnv)

			var got strings.Builder
			for _, d := range diags {
				if d.Severity == SeverityError && d.Line == 0 {
					t.Errorf("error without a line: %s", d)
				}
				fmt.Fprintln(&got, d)
			}
			if strings.Contains(got.String(), "s3cr3t-value") {
				t.Error("a diagnostic contains the value of a credential")
			}
			checkReference(t, filepath.Join(dir, "want.txt"), []byte(got.String()))
		})
	}
}

func TestLoadValidProject(t *testing.T) {
	p, diags := Load("testdata/load/valid", testEnv)
	if len(diags) != 0 {
		t.Fatalf("diagnostics = %v, want none", diags)
	}
	if p.Manifest != (Manifest{Name: "demo", Format: 1, Description: "Projeto de demonstração"}) {
		t.Errorf("manifest = %+v", p.Manifest)
	}

	var names []string
	for _, s := range p.Sources {
		names = append(names, s.Name)
	}
	if got, want := strings.Join(names, " "), "crm dw metas relatorio"; got != want {
		t.Fatalf("sources = %q, want %q", got, want)
	}

	crm := p.Sources[0]
	wantConnection := &PostgresConnection{Host: "crm.interno", Port: 5433, Database: "crm", User: "${CRM_USER}", Password: "${CRM_PASSWORD}"}
	if !reflect.DeepEqual(crm.Connection, wantConnection) {
		t.Errorf("crm connection = %#v, want %#v", crm.Connection, wantConnection)
	}
	if crm.Type != SourcePostgres || crm.Mode != ModeAttach || crm.Writable || strings.Join(crm.Tables, ",") != "clientes,contatos" {
		t.Errorf("crm = %+v", crm)
	}
	if !p.Sources[1].Writable {
		t.Error("dw.Writable = false, want true")
	}
	if got := p.Sources[3].Connection.(*FileConnection).Path; got != "files/relatório anual.pdf" {
		t.Errorf("relatorio path = %q", got)
	}
}

// sourceEdits are the changes applied to original.yaml in each directory of
// testdata/rewrite; the result must equal expected.yaml.
var sourceEdits = map[string]func(*Source){
	"unchanged": func(*Source) {},
	"change-scalars": func(s *Source) {
		c := s.Connection.(*PostgresConnection)
		c.Host = "novo-crm.interno"
		c.Port = 5432
		c.Password = "${CRM_NEW_PASSWORD}"
	},
	"remove-fields": func(s *Source) {
		s.Connection.(*PostgresConnection).Port = 0
	},
	"change-tables": func(s *Source) { s.Tables = []string{"clientes", "pedidos", "produtos"} },
	"remove-tables": func(s *Source) { s.Tables = nil },
	"change-type": func(s *Source) {
		s.Type = SourceCSV
		s.Connection = &FileConnection{Path: "files/clientes.csv"}
		s.Tables = nil
	},
	"add-fields": func(s *Source) {
		s.Description = "CRM: clientes e contatos"
		s.Writable = true
		s.Connection.(*PostgresConnection).Port = 5433
	},
	"add-tables": func(s *Source) { s.Tables = []string{"clientes"} },
	"four-space-indent": func(s *Source) {
		c := s.Connection.(*PostgresConnection)
		c.Host = "db2"
		c.Port = 6432
		s.Tables = []string{"clientes", "pedidos"}
	},
	"flow-style": func(s *Source) {
		s.Connection.(*FileConnection).Path = "b.csv"
		s.Tables = []string{"a"}
	},
	"no-final-newline": func(s *Source) { s.Connection.(*FileConnection).Path = "b.csv" },
	"comments-only": func(s *Source) {
		*s = Source{Type: SourceCSV, Mode: ModeAttach, Connection: &FileConnection{Path: "a.csv"}}
	},
	"empty": func(s *Source) {
		*s = Source{Type: SourceCSV, Mode: ModeAttach, Connection: &FileConnection{Path: "a.csv"}}
	},
	"multiline-description": func(s *Source) {
		s.Description = "Now a single line"
		s.Connection.(*FileConnection).Path = "b.csv"
	},
}

func rewriteSource(t *testing.T, original []byte, change func(*Source)) []byte {
	t.Helper()
	var before any
	var after Source
	if current, ok := decodeSourceLenient(original); ok {
		before = current
		after, _ = decodeSourceLenient(original) // decoded again: the connection is a pointer
	}
	change(&after)
	got, err := rewrite(original, before, after)
	if err != nil {
		t.Fatalf("rewrite() error = %v", err)
	}
	return got
}

func TestRewrite(t *testing.T) {
	const root = "testdata/rewrite"
	names := subdirectories(t, root)
	if len(names) != len(sourceEdits) {
		t.Errorf("testdata/rewrite has %d cases and sourceEdits has %d; they must match", len(names), len(sourceEdits))
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			change, ok := sourceEdits[name]
			if !ok {
				t.Fatalf("no entry in sourceEdits for %s", name)
			}
			original, err := os.ReadFile(filepath.Join(root, name, "original.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			got := rewriteSource(t, original, change)
			checkReference(t, filepath.Join(root, name, "expected.yaml"), got)

			// Whatever was written must read back as the edited source.
			var want Source
			if current, ok := decodeSourceLenient(original); ok {
				want = current
			}
			change(&want)
			if reread, ok := decodeSourceLenient(got); !ok || !reflect.DeepEqual(reread, want) {
				t.Errorf("reading the result back gives %+v, want %+v", reread, want)
			}
		})
	}
}

func TestRewriteKeepsWindowsLineEndings(t *testing.T) {
	original := []byte("type: csv\r\nmode: attach   # comment\r\nconnection:\r\n  path: a.csv\r\n")
	got := rewriteSource(t, original, func(s *Source) {
		s.Connection.(*FileConnection).Path = "b.csv"
		s.Tables = []string{"x"}
	})
	want := "type: csv\r\nmode: attach   # comment\r\nconnection:\r\n  path: b.csv\r\ntables:\r\n  - x\r\n"
	if string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRewriteRefusesInvalidYAML(t *testing.T) {
	_, err := rewrite([]byte("type: [csv\n"), nil, Source{Type: SourceCSV})
	if err == nil {
		t.Fatal("rewrite() error = nil, want an error instead of overwriting a file it cannot read")
	}
}

func copyDir(t *testing.T, from, to string) {
	t.Helper()
	if err := os.CopyFS(to, os.DirFS(from)); err != nil {
		t.Fatal(err)
	}
}

// Saving a source that did not change must not alter a single byte.
func TestWriteSourceKeepsUnchangedFilesIdentical(t *testing.T) {
	dir := t.TempDir()
	copyDir(t, "testdata/load/valid", dir)
	p, diags := Load(dir, testEnv)
	if len(diags) != 0 || len(p.Sources) == 0 {
		t.Fatalf("Load() = %d sources, diagnostics %v", len(p.Sources), diags)
	}

	for _, source := range p.Sources {
		file := filepath.Join(dir, "sources", source.Name+".yaml")
		before, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if err := WriteSource(dir, source); err != nil {
			t.Fatalf("WriteSource(%s) error = %v", source.Name, err)
		}
		after, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before, after) {
			t.Errorf("%s changed.\n--- before ---\n%s--- after ---\n%s", source.Name, before, after)
		}
	}
}

func TestWriteSourceCreatesAFileThatLoads(t *testing.T) {
	dir := t.TempDir()
	if err := Init(dir, "demo"); err != nil {
		t.Fatal(err)
	}
	source := Source{
		Name:        "crm",
		Description: "CRM",
		Type:        SourcePostgres,
		Mode:        ModeAttach,
		Connection:  &PostgresConnection{Host: "db", Database: "crm", User: "${CRM_USER}", Password: "${CRM_PASSWORD}"},
		Tables:      []string{"clientes"},
	}
	if err := WriteSource(dir, source); err != nil {
		t.Fatalf("WriteSource() error = %v", err)
	}

	p, diags := Load(dir, testEnv)
	if len(diags) != 0 {
		t.Fatalf("diagnostics = %v, want none", diags)
	}
	if len(p.Sources) != 1 || !reflect.DeepEqual(p.Sources[0], source) {
		t.Errorf("sources = %+v, want [%+v]", p.Sources, source)
	}

	leftovers, err := filepath.Glob(filepath.Join(dir, "sources", ".*"))
	if err != nil || len(leftovers) != 0 {
		t.Errorf("temporary files left behind: %v (%v)", leftovers, err)
	}
}

func TestWriteSourceRejectsNamesThatAreNotFileNames(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"", "../escape", "sub/dir", "Crm", "1st", "a b"} {
		err := WriteSource(dir, Source{Name: name, Type: SourceCSV, Mode: ModeAttach, Connection: &FileConnection{Path: "a.csv"}})
		if err == nil {
			t.Errorf("WriteSource(name %q) error = nil, want an error", name)
		}
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("files were created: %v", entries)
	}
}

func TestInit(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "novo")
	if err := Init(dir, "novo"); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	p, diags := Load(dir, testEnv)
	if len(diags) != 0 || p.Manifest != (Manifest{Name: "novo", Format: FormatVersion}) {
		t.Errorf("Load() = %+v, diagnostics %v", p.Manifest, diags)
	}
	ignore, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil || !strings.Contains(string(ignore), "data/\n") {
		t.Errorf(".gitignore = %q, %v; want it to ignore data/", ignore, err)
	}

	if err := Init(dir, "again"); err == nil {
		t.Error("second Init() error = nil, want an error instead of overwriting the project")
	}
}

func TestSchema(t *testing.T) {
	for _, kind := range []string{KindProject, KindSource} {
		t.Run(kind, func(t *testing.T) {
			got, err := Schema(kind)
			if err != nil {
				t.Fatalf("Schema() error = %v", err)
			}
			checkReference(t, filepath.Join("testdata", "schema", kind+".json"), got)
		})
	}
	if _, err := Schema("model"); err == nil {
		t.Error(`Schema("model") error = nil, want an error`)
	}
}

// sourceFile exists only for decoding; it must not drift from Source.
func TestSourceFileMirrorsSource(t *testing.T) {
	names := func(t reflect.Type) string {
		var out []string
		for _, f := range fieldsOfType(t) {
			out = append(out, f.name)
		}
		return strings.Join(out, " ")
	}
	if got, want := names(reflect.TypeFor[sourceFile]()), names(reflect.TypeFor[Source]()); got != want {
		t.Errorf("sourceFile fields = %q, Source fields = %q", got, want)
	}
}

func TestResolve(t *testing.T) {
	original := &PostgresConnection{Host: "${DW_HOST}:5432", Database: "crm", User: "${CRM_USER}", Password: "${CRM_PASSWORD}"}

	resolved, err := Resolve(original, testEnv)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	got := resolved.(*PostgresConnection)
	if got.Host != "dw.interno:5432" || got.User != "app" || got.Password.Reveal() != "s3cr3t-value" || got.Database != "crm" {
		t.Errorf("resolved = %#v", got)
	}
	if original.User != "${CRM_USER}" || original.Password != "${CRM_PASSWORD}" {
		t.Errorf("Resolve() modified the original: %#v", original)
	}

	_, err = Resolve(&PostgresConnection{User: "${NOT_SET}"}, testEnv)
	if err == nil || err.Error() != "connection.user: environment variable NOT_SET is not set" {
		t.Errorf("Resolve() error = %v", err)
	}
}

func TestSecretNeverPrintsItsValue(t *testing.T) {
	conn := &PostgresConnection{Host: "db", Password: "s3cr3t-value"}

	var logged bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logged, nil))
	logger.Info("connecting", "password", conn.Password, "connection", conn)

	outputs := []string{
		fmt.Sprint(conn.Password),
		fmt.Sprintf("%s %v %+v %#v %q %x", conn.Password, conn.Password, conn, conn, conn.Password, conn.Password),
		fmt.Errorf("connect with %v: failed", conn).Error(),
		logged.String(),
	}
	for _, out := range outputs {
		if strings.Contains(out, "s3cr3t") || strings.Contains(out, fmt.Sprintf("%x", "s3cr3t")) {
			t.Errorf("the value leaked: %s", out)
		}
	}
}

func TestReferences(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"plain text", "", false},
		{"${A}", "A", false},
		{"${A_1}:${b}", "A_1 b", false},
		{"$A and $ {A}", "", false},
		{"${a-b}", "", true},
		{"${}", "", true},
		{"${1A}", "", true},
		{"${A", "", true},
		{"${A} ${B", "", true},
	}
	for _, tt := range tests {
		names, err := references(tt.in)
		if got := strings.Join(names, " "); got != tt.want || (err != nil) != tt.wantErr {
			t.Errorf("references(%q) = %q, %v; want %q, error %v", tt.in, got, err, tt.want, tt.wantErr)
		}
	}
}
