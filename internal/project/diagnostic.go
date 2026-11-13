package project

import (
	"fmt"
	"strings"
)

// Severity tells a problem that makes the project unusable from one that is
// only worth knowing about.
type Severity int

const (
	SeverityError Severity = iota
	SeverityWarning
)

func (s Severity) String() string {
	if s == SeverityWarning {
		return "warning"
	}
	return "error"
}

// Diagnostic is one problem found in a project file.
type Diagnostic struct {
	Severity Severity
	// File is relative to the project directory and uses forward slashes.
	File string
	// Line and Column are 1-based. Errors always have a line; a warning about
	// a whole directory has none (0).
	Line   int
	Column int
	// Field is the dotted path of the field, such as "connection.password".
	// It is empty when the problem is not about one field.
	Field   string
	Message string
}

// String formats the diagnostic the way compilers do, so that editors and
// terminals can jump to the position: "file:line:column: error: field: message".
func (d Diagnostic) String() string {
	var b strings.Builder
	b.WriteString(d.File)
	if d.Line > 0 {
		fmt.Fprintf(&b, ":%d:%d", d.Line, d.Column)
	}
	fmt.Fprintf(&b, ": %s: ", d.Severity)
	if d.Field != "" {
		fmt.Fprintf(&b, "%s: ", d.Field)
	}
	b.WriteString(d.Message)
	return b.String()
}

// Diagnostics is everything found while loading a project.
type Diagnostics []Diagnostic

// Errors counts the diagnostics that are errors.
func (ds Diagnostics) Errors() int {
	n := 0
	for _, d := range ds {
		if d.Severity == SeverityError {
			n++
		}
	}
	return n
}
