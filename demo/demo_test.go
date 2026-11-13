package demo

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/JoaoGobboo/intarsia/internal/project"
)

var update = flag.Bool("update", false, "rewrite testdata/checksums.txt with the current output")

func TestGenerateIsAlwaysTheSame(t *testing.T) {
	if first, second := Generate(), Generate(); !reflect.DeepEqual(first, second) {
		t.Fatal("Generate() returned different data on two calls")
	}
}

// The checksums pin the generated files byte for byte. If this fails without a
// deliberate change to the generator, the data stopped being reproducible,
// for instance across platforms or Go versions.
func TestFilesMatchTheirChecksums(t *testing.T) {
	company := Generate()
	var got strings.Builder
	for _, file := range Files() {
		data, err := file.Contents(company)
		if err != nil {
			t.Fatalf("%s: %v", file.Path, err)
		}
		fmt.Fprintf(&got, "%x  %s\n", sha256.Sum256(data), file.Path)
	}

	const reference = "testdata/checksums.txt"
	if *update {
		if err := os.WriteFile(reference, []byte(got.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(reference)
	if err != nil {
		t.Fatalf("%v (run `go test ./demo -update` to create it)", err)
	}
	if got.String() != string(want) {
		t.Errorf("generated files changed.\n--- got ---\n%s--- want ---\n%s", got.String(), want)
	}
}

func TestWriteTwiceProducesTheSameFiles(t *testing.T) {
	read := func(dir string) map[string][]byte {
		t.Helper()
		if err := Write(dir); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		files := map[string][]byte{}
		for _, file := range Files() {
			data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(file.Path)))
			if err != nil {
				t.Fatal(err)
			}
			files[file.Path] = data
		}
		return files
	}

	dir := t.TempDir()
	first := read(dir)
	second := read(dir) // over the existing files
	third := read(t.TempDir())
	if !reflect.DeepEqual(first, second) || !reflect.DeepEqual(first, third) {
		t.Error("Write() produced different files on different runs")
	}
}

// The defects are the point of the data; a refactoring must not lose them.
func TestTheDataHasItsDeliberateDefects(t *testing.T) {
	c := Generate()
	if len(c.Customers) != customerCount {
		t.Fatalf("customers = %d, want %d", len(c.Customers), customerCount)
	}

	byCNPJ := map[string][]Customer{}
	for _, customer := range c.Customers {
		byCNPJ[digitsOnly(customer.CNPJ)] = append(byCNPJ[digitsOnly(customer.CNPJ)], customer)
	}
	duplicated := 0
	for _, same := range byCNPJ {
		if len(same) == 2 && same[0].Name != same[1].Name && strings.EqualFold(same[0].Name+".", same[1].Name) {
			duplicated++
		}
	}
	if duplicated != duplicateCount {
		t.Errorf("customers registered twice with different spellings = %d, want %d", duplicated, duplicateCount)
	}

	names := map[string]string{}
	for _, customer := range c.Customers {
		names[fmt.Sprintf("C-%04d", customer.ID)] = customer.Name
	}
	var orphans, respelled, monthSlash, monthISO int
	for _, target := range c.Targets {
		name, known := names[target.CustomerCode]
		switch {
		case !known:
			orphans++
		case name != target.CustomerName:
			respelled++
		}
		switch {
		case regexp.MustCompile(`^\d{2}/\d{4}$`).MatchString(target.Month):
			monthSlash++
		case regexp.MustCompile(`^\d{4}-\d{2}$`).MatchString(target.Month):
			monthISO++
		default:
			t.Errorf("unexpected month format %q", target.Month)
		}
	}
	if orphans != orphanCount {
		t.Errorf("targets for customers that do not exist = %d, want %d", orphans, orphanCount)
	}
	if respelled == 0 || monthSlash == 0 || monthISO == 0 {
		t.Errorf("respelled names = %d, months as MM/YYYY = %d, months as YYYY-MM = %d; want all above zero",
			respelled, monthSlash, monthISO)
	}

	respelledSalespeople := 0
	for _, s := range c.Salespeople {
		if s.Name != s.PDFName {
			respelledSalespeople++
		}
	}
	if respelledSalespeople != 1 {
		t.Errorf("salespeople spelled differently in the PDF = %d, want 1", respelledSalespeople)
	}
}

func TestProjectIsValid(t *testing.T) {
	env := func(name string) (string, bool) {
		value, ok := map[string]string{"CRM_USER": CRMUser, "CRM_PASSWORD": CRMPassword}[name]
		return value, ok
	}
	p, diagnostics := project.Load("project", env)
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %v, want none", diagnostics)
	}
	var names []string
	for _, source := range p.Sources {
		names = append(names, source.Name)
	}
	if got, want := strings.Join(names, " "), "comissoes crm metas"; got != want {
		t.Errorf("sources = %q, want %q", got, want)
	}
}

func TestSpreadsheetReadsBack(t *testing.T) {
	var file bytes.Buffer
	rows := [][]any{{"nome", "valor"}, {" Farmácia <A&B>  ", 1500.5}, {"nome", 2.0}}
	if err := writeXLSX(&file, "Planilha", rows); err != nil {
		t.Fatalf("writeXLSX() error = %v", err)
	}

	archive, err := zip.NewReader(bytes.NewReader(file.Bytes()), int64(file.Len()))
	if err != nil {
		t.Fatalf("not a zip file: %v", err)
	}
	part := func(name string, into any) {
		t.Helper()
		f, err := archive.Open(name)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = f.Close() }()
		data, err := io.ReadAll(f)
		if err != nil {
			t.Fatal(err)
		}
		if err := xml.Unmarshal(data, into); err != nil {
			t.Fatalf("%s is not valid XML: %v", name, err)
		}
	}

	var shared struct {
		Strings []string `xml:"si>t"`
	}
	var sheet struct {
		Rows []struct {
			Cells []struct {
				Ref   string `xml:"r,attr"`
				Type  string `xml:"t,attr"`
				Value string `xml:"v"`
			} `xml:"c"`
		} `xml:"sheetData>row"`
	}
	part("xl/sharedStrings.xml", &shared)
	part("xl/worksheets/sheet1.xml", &sheet)

	var got [][]any
	for _, row := range sheet.Rows {
		var cells []any
		for _, cell := range row.Cells {
			if cell.Type == "s" {
				i, err := strconv.Atoi(cell.Value)
				if err != nil || i >= len(shared.Strings) {
					t.Fatalf("cell %s refers to shared string %q", cell.Ref, cell.Value)
				}
				cells = append(cells, shared.Strings[i])
				continue
			}
			number, err := strconv.ParseFloat(cell.Value, 64)
			if err != nil {
				t.Fatalf("cell %s: %v", cell.Ref, err)
			}
			cells = append(cells, number)
		}
		got = append(got, cells)
	}
	if !reflect.DeepEqual(got, rows) {
		t.Errorf("read back %v, want %v", got, rows)
	}
	if len(shared.Strings) != 3 {
		t.Errorf("shared strings = %q, want each distinct string once", shared.Strings)
	}
}

// A PDF reader finds every object through the cross-reference table, so a
// wrong offset there makes the file unreadable.
func TestPDFCrossReferenceTableIsExact(t *testing.T) {
	var file bytes.Buffer
	if err := writePDF(&file, "Comissões (2026)", []float64{50, 300}, [][]string{{"Vendedor", "Região"}, {"João", "Sul"}}); err != nil {
		t.Fatalf("writePDF() error = %v", err)
	}
	data := file.Bytes()
	if !bytes.HasPrefix(data, []byte("%PDF-1.4\n")) || !bytes.HasSuffix(data, []byte("%%EOF\n")) {
		t.Fatal("the file does not start and end like a PDF")
	}

	start := regexp.MustCompile(`startxref\n(\d+)\n%%EOF\n$`).FindSubmatch(data)
	if start == nil {
		t.Fatal("no startxref")
	}
	xref, _ := strconv.Atoi(string(start[1]))
	if !bytes.HasPrefix(data[xref:], []byte("xref\n0 7\n0000000000 65535 f \n")) {
		t.Fatalf("startxref does not point to the table: %q", data[xref:min(xref+40, len(data))])
	}
	entries := regexp.MustCompile(`(?m)^(\d{10}) 00000 n $`).FindAllSubmatch(data[xref:], -1)
	if len(entries) != 6 {
		t.Fatalf("table has %d objects, want 6", len(entries))
	}
	for i, entry := range entries {
		offset, _ := strconv.Atoi(string(entry[1]))
		if want := fmt.Sprintf("%d 0 obj\n", i+1); !bytes.HasPrefix(data[offset:], []byte(want)) {
			t.Errorf("object %d: offset %d does not point to it", i+1, offset)
		}
	}

	stream := regexp.MustCompile(`(?s)/Length (\d+) >>\nstream\n(.*)endstream`).FindSubmatch(data)
	if length, _ := strconv.Atoi(string(stream[1])); length != len(stream[2]) {
		t.Errorf("/Length = %d, stream has %d bytes", length, len(stream[2]))
	}
	// Accents are one WinAnsi byte each; parentheses are escaped.
	for _, want := range []string{"(Comiss\xf5es \\(2026\\))", "(Jo\xe3o)", "(Regi\xe3o)"} {
		if !bytes.Contains(stream[2], []byte(want)) {
			t.Errorf("page does not contain %q", want)
		}
	}
}

func TestCRMScriptEscapesQuotes(t *testing.T) {
	var script bytes.Buffer
	c := Company{Customers: []Customer{{ID: 1, Name: "Bar d'Água Ltda"}}}
	if err := writeCRM(&script, c); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(script.String(), "(1, 'Bar d''Água Ltda', ") {
		t.Errorf("name not escaped:\n%s", script.String())
	}
	if !strings.HasSuffix(script.String(), ");\n") {
		t.Error("script does not end the insert statement")
	}
}
