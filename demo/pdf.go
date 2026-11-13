package demo

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// writeCommissions writes the commission table: a one-page PDF with a title
// and a ruled table, the kind of document that exists only as a PDF.
func writeCommissions(w io.Writer, c Company) error {
	rows := [][]string{{"Vendedor", "Região", "Comissão", "Vigência"}}
	for _, s := range c.Salespeople {
		commission := strings.Replace(strconv.FormatFloat(s.Commission, 'f', 1, 64), ".", ",", 1) + "%"
		rows = append(rows, []string{s.PDFName, s.Region, commission, s.Since.Format("02/01/2006")})
	}
	return writePDF(w, fmt.Sprintf("Tabela de comissões por vendedor - %d", targetYear), []float64{50, 250, 380, 470}, rows)
}

// Page geometry, in points. The origin of a PDF page is its bottom left corner.
const (
	pageWidth, pageHeight = 595, 842 // A4
	marginLeft            = 50
	marginRight           = pageWidth - 50
	titleY                = 780
	tableTop              = 740
	rowHeight             = 22
	textBaseline          = 7 // from the bottom rule of a row to its text
)

// writePDF writes a page with a title and a table. columns are the x positions
// of the columns; the first row is the header.
//
// The PDF is written directly, object by object: a text-only page needs very
// little of the format, and the result has no creation date or random
// identifier, so it is identical on every run.
func writePDF(w io.Writer, title string, columns []float64, rows [][]string) error {
	var page strings.Builder
	text := func(font string, size int, x, y float64, s string) {
		fmt.Fprintf(&page, "BT /%s %d Tf %s %s Td (%s) Tj ET\n", font, size, number(x), number(y), pdfString(s))
	}
	rule := func(y float64) {
		fmt.Fprintf(&page, "%d %s m %d %s l S\n", marginLeft, number(y), marginRight, number(y))
	}

	text("F2", 14, marginLeft, titleY, title)
	page.WriteString("0.5 w\n")
	rule(tableTop)
	for r, row := range rows {
		bottom := float64(tableTop - (r+1)*rowHeight)
		font := "F1"
		if r == 0 {
			font = "F2"
		}
		for col, cell := range row {
			text(font, 10, columns[col], bottom+textBaseline, cell)
		}
		rule(bottom)
	}

	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %d %d] "+
			"/Resources << /Font << /F1 4 0 R /F2 5 0 R >> >> /Contents 6 0 R >>", pageWidth, pageHeight),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", page.Len(), page.String()),
	}

	var out bytes.Buffer
	// The second line is a comment with bytes above 127: the convention that
	// tells tools the file is binary.
	out.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
	offsets := make([]int, len(objects))
	for i, object := range objects {
		offsets[i] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	// The cross-reference table gives the byte offset of every object. Each
	// entry is exactly 20 bytes long.
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, offset := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)

	_, err := w.Write(out.Bytes())
	return err
}

func number(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

// pdfString encodes text for a PDF string between parentheses. The standard
// fonts take one byte per character in WinAnsi, which matches Unicode for
// every accented letter of Portuguese.
func pdfString(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\\' || r == '(' || r == ')':
			b.WriteByte('\\')
			b.WriteByte(byte(r))
		case r >= 0x20 && r < 0x7f, r >= 0xa0 && r <= 0xff:
			b.WriteByte(byte(r))
		default:
			b.WriteByte('?')
		}
	}
	return b.String()
}
