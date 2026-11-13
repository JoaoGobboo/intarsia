package project

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/goccy/go-yaml/token"
)

// document is a parsed project file. It answers "where is this field?", which
// is what lets every validation error point at a line.
type document struct {
	file string
	root *ast.MappingNode
}

func parseDocument(file string, data []byte) (*document, Diagnostics) {
	doc := &document{file: file}
	parsed, err := parseYAML(data)
	if err != nil {
		return nil, Diagnostics{doc.decodeFailure(err)}
	}
	if len(parsed.Docs) > 1 {
		return nil, Diagnostics{doc.at(1, 1, "", "the file has more than one YAML document; keep a single one")}
	}

	var body ast.Node
	if len(parsed.Docs) == 1 {
		body = parsed.Docs[0].Body
	}
	switch body := body.(type) {
	case *ast.MappingNode:
		doc.root = body
		return doc, nil
	case nil, *ast.CommentGroupNode:
		return nil, Diagnostics{doc.at(1, 1, "", "the file is empty")}
	default:
		pos := body.GetToken().Position
		return nil, Diagnostics{doc.at(pos.Line, pos.Column, "", "expected fields in the form `name: value`")}
	}
}

// parseYAML parses with Windows line endings converted first: the parser
// miscounts lines after a comment that ends in CRLF. Line and column numbers
// are the same in both forms, so positions still refer to the original text.
func parseYAML(data []byte) (*ast.File, error) {
	return parser.ParseBytes(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n")), parser.ParseComments)
}

func (d *document) at(line, column int, fieldPath, message string) Diagnostic {
	return Diagnostic{File: d.file, Line: line, Column: column, Field: fieldPath, Message: message}
}

// invalid reports a problem with a field. When the field is absent, the
// position is the closest enclosing field that exists.
func (d *document) invalid(fieldPath, message string) Diagnostic {
	line, column := 1, 1
	mapping := d.root
	for name := range strings.SplitSeq(fieldPath, ".") {
		entry := findEntry(mapping, name)
		if entry == nil {
			break
		}
		pos := entry.Key.GetToken().Position
		if _, scalar := entry.Value.(ast.ScalarNode); scalar && entry.Value.GetToken() != nil {
			pos = entry.Value.GetToken().Position
		}
		line, column = pos.Line, pos.Column
		mapping, _ = entry.Value.(*ast.MappingNode)
	}
	return d.at(line, column, fieldPath, message)
}

func findEntry(mapping *ast.MappingNode, key string) *ast.MappingValueNode {
	if mapping == nil {
		return nil
	}
	for _, entry := range mapping.Values {
		if entry.Key.GetToken().Value == key {
			return entry
		}
	}
	return nil
}

// decodeFailure turns an error of the YAML library into a diagnostic.
func (d *document) decodeFailure(err error) Diagnostic {
	var located yaml.Error
	if !errors.As(err, &located) || located.GetToken() == nil {
		return d.at(1, 1, "", err.Error())
	}
	tok := located.GetToken()
	message := located.GetMessage()

	var wrongType *yaml.TypeError
	if errors.As(err, &wrongType) {
		message = "expected " + describeKind(wrongType.DstType)
	}
	var unknown *yaml.UnknownFieldError
	if errors.As(err, &unknown) {
		message = "unknown field"
	}
	return d.at(tok.Position.Line, tok.Position.Column, d.fieldAt(d.root, "", tok), message)
}

// fieldAt finds the dotted path of the field that owns a token.
func (d *document) fieldAt(mapping *ast.MappingNode, prefix string, tok *token.Token) string {
	if mapping == nil {
		return ""
	}
	owns := func(node ast.Node) bool {
		t := node.GetToken()
		return t != nil && t.Position.Line == tok.Position.Line && t.Position.Column == tok.Position.Column
	}
	for _, entry := range mapping.Values {
		fieldPath := prefix + entry.Key.GetToken().Value
		if owns(entry.Key) || owns(entry.Value) {
			return fieldPath
		}
		switch value := entry.Value.(type) {
		case *ast.MappingNode:
			if found := d.fieldAt(value, fieldPath+".", tok); found != "" {
				return found
			}
		case *ast.SequenceNode:
			if slices.ContainsFunc(value.Values, owns) {
				return fieldPath
			}
		}
	}
	return ""
}

func describeKind(t reflect.Type) string {
	switch t.Kind() {
	case reflect.Bool:
		return "true or false"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "a whole number"
	case reflect.String:
		return "text"
	case reflect.Slice:
		return "a list"
	case reflect.Map, reflect.Struct, reflect.Pointer:
		return "a group of fields"
	default:
		return "a different kind of value"
	}
}

// enumerated is implemented by field types that accept a fixed set of values.
type enumerated interface {
	Values() []string
}

// check validates what the YAML library cannot know: required fields and
// fields with a fixed set of values. v is a struct; prefix is its field path.
func (d *document) check(v reflect.Value, prefix string) Diagnostics {
	var diags Diagnostics
	for _, f := range fieldsOf(v) {
		fieldPath := prefix + f.name
		if f.value.IsZero() {
			if f.required {
				diags = append(diags, d.invalid(fieldPath, "is required"))
			}
			continue
		}
		if enum, ok := f.value.Interface().(enumerated); ok && !slices.Contains(enum.Values(), f.value.String()) {
			diags = append(diags, d.invalid(fieldPath, "must be one of: "+strings.Join(enum.Values(), ", ")))
		}
	}
	return diags
}

func decodeManifest(file string, data []byte) (Manifest, Diagnostics) {
	var manifest Manifest
	doc, diags := parseDocument(file, data)
	if doc == nil {
		return manifest, diags
	}
	if err := yaml.NodeToValue(doc.root, &manifest, yaml.Strict()); err != nil {
		return manifest, Diagnostics{doc.decodeFailure(err)}
	}

	diags = doc.check(reflect.ValueOf(manifest), "")
	if manifest.Format != 0 && manifest.Format != FormatVersion {
		diags = append(diags, doc.invalid("format", fmt.Sprintf(
			"unsupported format version %d: this version of Intársia reads format %d", manifest.Format, FormatVersion)))
	}
	return manifest, diags
}

// sourceFile mirrors Source for decoding. The connection fields depend on the
// type, so they are decoded in a second step, once the type is known.
type sourceFile struct {
	Description string         `yaml:"description"`
	Type        SourceType     `yaml:"type"`
	Mode        Mode           `yaml:"mode"`
	Writable    bool           `yaml:"writable"`
	Connection  map[string]any `yaml:"connection"`
	Tables      []string       `yaml:"tables"`
}

func (f sourceFile) source(name string) Source {
	return Source{Name: name, Description: f.Description, Type: f.Type, Mode: f.Mode, Writable: f.Writable, Tables: f.Tables}
}

func decodeSource(name, file string, data []byte, lookup LookupFunc) (Source, Diagnostics) {
	doc, diags := parseDocument(file, data)
	if doc == nil {
		return Source{Name: name}, diags
	}
	if !itemName.MatchString(name) {
		diags = append(diags, doc.at(1, 1, "", fmt.Sprintf("invalid file name: the name of a source becomes its name in queries, so %s", itemNameRule)))
	}

	var raw sourceFile
	if err := yaml.NodeToValue(doc.root, &raw, yaml.Strict()); err != nil {
		return Source{Name: name}, append(diags, doc.decodeFailure(err))
	}
	source := raw.source(name)
	source.Connection = NewConnection(source.Type)

	// The connection is checked below, field by field; here it only has to exist.
	header := source
	header.Connection = &FileConnection{}
	diags = append(diags, doc.check(reflect.ValueOf(header), "")...)
	diags = append(diags, checkTables(doc, source.Tables)...)

	entry := findEntry(doc.root, "connection")
	switch {
	case entry == nil || raw.Connection == nil:
		diags = append(diags, doc.invalid("connection", "is required"))
	case source.Connection != nil:
		if err := yaml.NodeToValue(entry.Value, source.Connection, yaml.Strict()); err != nil {
			return source, append(diags, doc.decodeFailure(err))
		}
		diags = append(diags, checkConnection(doc, source, lookup)...)
	}
	return source, diags
}

// decodeSourceLenient reads a source without validating it, ignoring what it
// does not understand. It reports false when the text cannot be read at all.
func decodeSourceLenient(data []byte) (Source, bool) {
	doc, _ := parseDocument("", data)
	if doc == nil {
		return Source{}, false
	}
	var raw sourceFile
	if err := yaml.NodeToValue(doc.root, &raw); err != nil {
		return Source{}, false
	}
	source := raw.source("")
	source.Connection = NewConnection(source.Type)
	if entry := findEntry(doc.root, "connection"); entry != nil && source.Connection != nil {
		if err := yaml.NodeToValue(entry.Value, source.Connection); err != nil {
			return Source{}, false
		}
	}
	return source, true
}

func checkTables(doc *document, tables []string) Diagnostics {
	var diags Diagnostics
	seen := make(map[string]bool, len(tables))
	for _, table := range tables {
		switch {
		case strings.TrimSpace(table) == "":
			diags = append(diags, doc.invalid("tables", "a table name cannot be empty"))
		case seen[table]:
			diags = append(diags, doc.invalid("tables", fmt.Sprintf("table %q is listed more than once", table)))
		}
		seen[table] = true
	}
	return diags
}

var secretType = reflect.TypeFor[Secret]()

// checkConnection validates the connection fields and the variables they
// reference. It reports variable names only, never their values.
func checkConnection(doc *document, source Source, lookup LookupFunc) Diagnostics {
	connection := reflect.ValueOf(source.Connection).Elem()
	diags := doc.check(connection, "connection.")

	for _, f := range fieldsOf(connection) {
		if f.value.Kind() != reflect.String || f.value.IsZero() {
			continue
		}
		fieldPath := "connection." + f.name
		names, err := references(f.value.String())
		if err != nil {
			diags = append(diags, doc.invalid(fieldPath, err.Error()))
			continue
		}
		if f.typ == secretType && (len(names) != 1 || f.value.String() != "${"+names[0]+"}") {
			example := strings.ToUpper(source.Name + "_" + f.name)
			diags = append(diags, doc.invalid(fieldPath, fmt.Sprintf(
				"must be a variable reference such as ${%s}: credentials are never written in project files", example)))
			continue
		}
		for _, name := range names {
			if _, ok := lookup(name); !ok {
				diags = append(diags, doc.invalid(fieldPath, fmt.Sprintf("environment variable %s is not set", name)))
			}
		}
	}
	return diags
}
