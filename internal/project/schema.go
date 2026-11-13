package project

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
)

// The kinds of project file that have a schema.
const (
	KindProject = "project"
	KindSource  = "source"
)

// Schema returns the JSON Schema of a kind of project file, generated from the
// Go types that read it. The interface builds its forms from it.
func Schema(kind string) ([]byte, error) {
	var root *schema
	switch kind {
	case KindProject:
		root = schemaOf(reflect.TypeFor[Manifest]())
	case KindSource:
		root = schemaOf(reflect.TypeFor[Source]())
		// The fields of connection depend on the type of the source.
		for _, name := range SourceType("").Values() {
			connection := reflect.TypeOf(NewConnection(SourceType(name))).Elem()
			root.AllOf = append(root.AllOf, conditional{
				If:   &schema{Properties: properties{{"type", &schema{Const: name}}}},
				Then: &schema{Properties: properties{{"connection", schemaOf(connection)}}},
			})
		}
	default:
		return nil, fmt.Errorf("no schema for %q: the kinds are %s and %s", kind, KindProject, KindSource)
	}
	root.Schema = "https://json-schema.org/draft/2020-12/schema"

	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(root); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// schema is the subset of JSON Schema that the project files need.
type schema struct {
	Schema               string        `json:"$schema,omitempty"`
	Type                 string        `json:"type,omitempty"`
	Format               string        `json:"format,omitempty"`
	Const                string        `json:"const,omitempty"`
	Enum                 []string      `json:"enum,omitempty"`
	Items                *schema       `json:"items,omitempty"`
	Properties           properties    `json:"properties,omitempty"`
	Required             []string      `json:"required,omitempty"`
	AdditionalProperties *bool         `json:"additionalProperties,omitempty"`
	AllOf                []conditional `json:"allOf,omitempty"`
}

type conditional struct {
	If   *schema `json:"if"`
	Then *schema `json:"then"`
}

type property struct {
	name   string
	schema *schema
}

// properties keeps the declaration order, which a Go map would lose: forms
// show the fields in the order of the schema.
type properties []property

func (p properties) MarshalJSON() ([]byte, error) {
	var out bytes.Buffer
	out.WriteByte('{')
	for i, prop := range p {
		if i > 0 {
			out.WriteByte(',')
		}
		name, err := json.Marshal(prop.name)
		if err != nil {
			return nil, err
		}
		value, err := json.Marshal(prop.schema)
		if err != nil {
			return nil, err
		}
		out.Write(name)
		out.WriteByte(':')
		out.Write(value)
	}
	out.WriteByte('}')
	return out.Bytes(), nil
}

var enumeratedType = reflect.TypeFor[enumerated]()

func schemaOf(t reflect.Type) *schema {
	switch {
	case t == secretType:
		return &schema{Type: "string", Format: "password"}
	case t.Implements(enumeratedType):
		return &schema{Type: "string", Enum: reflect.Zero(t).Interface().(enumerated).Values()}
	}

	switch t.Kind() {
	case reflect.String:
		return &schema{Type: "string"}
	case reflect.Bool:
		return &schema{Type: "boolean"}
	case reflect.Int:
		return &schema{Type: "integer"}
	case reflect.Slice:
		return &schema{Type: "array", Items: schemaOf(t.Elem())}
	case reflect.Interface:
		// Connection: an object whose fields are given by the conditionals.
		return &schema{Type: "object"}
	case reflect.Struct:
		closed := false
		s := &schema{Type: "object", AdditionalProperties: &closed}
		for _, f := range fieldsOfType(t) {
			s.Properties = append(s.Properties, property{f.name, schemaOf(f.typ)})
			if f.required {
				s.Required = append(s.Required, f.name)
			}
		}
		return s
	default:
		panic(fmt.Sprintf("project: no JSON Schema for Go type %s", t))
	}
}
