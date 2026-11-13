package demo

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// writeTargets writes the sales targets spreadsheet.
func writeTargets(w io.Writer, c Company) error {
	rows := [][]any{{"codigo_cliente", "cliente", "vendedor", "mes", "meta"}}
	for _, t := range c.Targets {
		rows = append(rows, []any{t.CustomerCode, t.CustomerName, t.Salesperson, t.Month, t.Amount})
	}
	return writeXLSX(w, "Metas", rows)
}

// writeXLSX writes a workbook with one sheet. A cell is a string or a float64.
//
// An .xlsx file is a zip of XML files, and the few written here are all a
// workbook needs. Writing them directly, instead of through a library, is what
// keeps the file identical on every run: no creation date, no compression
// whose output could change between versions.
func writeXLSX(w io.Writer, sheet string, rows [][]any) error {
	if len(rows) == 0 || len(rows[0]) > 26 {
		return fmt.Errorf("demo: a sheet needs at least one row and at most 26 columns")
	}

	// Strings are stored once in a shared table and cells refer to them by
	// index, which is how Excel itself writes them.
	var shared []string
	index := map[string]int{}
	var data strings.Builder
	for r, row := range rows {
		fmt.Fprintf(&data, `<row r="%d">`, r+1)
		for col, cell := range row {
			ref := string(rune('A'+col)) + strconv.Itoa(r+1)
			switch value := cell.(type) {
			case string:
				i, ok := index[value]
				if !ok {
					i = len(shared)
					index[value] = i
					shared = append(shared, value)
				}
				fmt.Fprintf(&data, `<c r="%s" t="s"><v>%d</v></c>`, ref, i)
			case float64:
				fmt.Fprintf(&data, `<c r="%s"><v>%s</v></c>`, ref, strconv.FormatFloat(value, 'f', -1, 64))
			default:
				return fmt.Errorf("demo: cell %s has unsupported type %T", ref, cell)
			}
		}
		data.WriteString(`</row>`)
	}

	var strs strings.Builder
	for _, s := range shared {
		// xml:space keeps the leading and trailing spaces some names have.
		fmt.Fprintf(&strs, `<si><t xml:space="preserve">%s</t></si>`, escapeXML(s))
	}

	const (
		header   = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n"
		mainNS   = `xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"`
		relNS    = `xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"`
		relTypes = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/"
		contents = "application/vnd.openxmlformats-officedocument.spreadsheetml."
	)
	files := []struct{ name, body string }{
		{"[Content_Types].xml", `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
			`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
			`<Default Extension="xml" ContentType="application/xml"/>` +
			`<Override PartName="/xl/workbook.xml" ContentType="` + contents + `sheet.main+xml"/>` +
			`<Override PartName="/xl/worksheets/sheet1.xml" ContentType="` + contents + `worksheet+xml"/>` +
			`<Override PartName="/xl/sharedStrings.xml" ContentType="` + contents + `sharedStrings+xml"/>` +
			`</Types>`},
		{"_rels/.rels", `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="` + relTypes + `officeDocument" Target="xl/workbook.xml"/>` +
			`</Relationships>`},
		{"xl/workbook.xml", `<workbook ` + mainNS + ` ` + relNS + `>` +
			`<sheets><sheet name="` + escapeXML(sheet) + `" sheetId="1" r:id="rId1"/></sheets>` +
			`</workbook>`},
		{"xl/_rels/workbook.xml.rels", `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="` + relTypes + `worksheet" Target="worksheets/sheet1.xml"/>` +
			`<Relationship Id="rId2" Type="` + relTypes + `sharedStrings" Target="sharedStrings.xml"/>` +
			`</Relationships>`},
		{"xl/worksheets/sheet1.xml", `<worksheet ` + mainNS + `><sheetData>` + data.String() + `</sheetData></worksheet>`},
		{"xl/sharedStrings.xml", fmt.Sprintf(`<sst %s count="%d" uniqueCount="%d">%s</sst>`, mainNS, len(shared), len(shared), strs.String())},
	}

	archive := zip.NewWriter(w)
	for _, file := range files {
		entry, err := archive.CreateHeader(&zip.FileHeader{
			Name:     file.name,
			Method:   zip.Store,
			Modified: time.Date(targetYear, time.January, 1, 0, 0, 0, 0, time.UTC),
		})
		if err != nil {
			return err
		}
		if _, err := io.WriteString(entry, header+file.body); err != nil {
			return err
		}
	}
	return archive.Close()
}

func escapeXML(s string) string {
	var b bytes.Buffer
	// EscapeText only fails when the writer does, and a buffer does not.
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
