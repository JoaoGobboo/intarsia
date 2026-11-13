package project

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
	"regexp"
	"strings"
)

// LookupFunc reads an environment variable. os.LookupEnv is the usual one.
type LookupFunc func(name string) (value string, ok bool)

// Secret is a credential. In a project file it holds a variable reference such
// as ${CRM_PASSWORD}; after Resolve it holds the real value. A reference is
// not sensitive and prints as it is. Anything else prints as [redacted] in
// formatting, logging and JSON; Reveal is the only way to read it.
type Secret string

const redacted = "[redacted]"

var secretReference = regexp.MustCompile(`^\$\{[A-Za-z_][A-Za-z0-9_]*\}$`)

func (s Secret) printable() string {
	if secretReference.MatchString(string(s)) {
		return string(s)
	}
	return redacted
}

// Format implements fmt.Formatter, covering every verb (%v, %s, %q, %#v...).
func (s Secret) Format(f fmt.State, _ rune) { _, _ = fmt.Fprint(f, s.printable()) }

// LogValue implements slog.LogValuer.
func (s Secret) LogValue() slog.Value { return slog.StringValue(s.printable()) }

// MarshalJSON implements json.Marshaler.
func (s Secret) MarshalJSON() ([]byte, error) { return json.Marshal(s.printable()) }

// Reveal returns the value. Never pass the result to a log or an error.
func (s Secret) Reveal() string { return string(s) }

var (
	variableReference = regexp.MustCompile(`\$\{([^}]*)\}`)
	variableName      = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

// references lists the variables named in s, in order of appearance.
func references(s string) ([]string, error) {
	var names []string
	for _, match := range variableReference.FindAllStringSubmatch(s, -1) {
		if !variableName.MatchString(match[1]) {
			return nil, fmt.Errorf("invalid variable reference %s: use letters, digits and underscores", match[0])
		}
		names = append(names, match[1])
	}
	if strings.Contains(variableReference.ReplaceAllString(s, ""), "${") {
		return nil, fmt.Errorf("variable reference is missing the closing brace")
	}
	return names, nil
}

// expand replaces every ${NAME} in s. The error names the variable, never a
// value.
func expand(s string, lookup LookupFunc) (string, error) {
	names, err := references(s)
	if err != nil {
		return "", err
	}
	for _, name := range names {
		value, ok := lookup(name)
		if !ok {
			return "", fmt.Errorf("environment variable %s is not set", name)
		}
		s = strings.ReplaceAll(s, "${"+name+"}", value)
	}
	return s, nil
}

// Resolve returns a copy of the connection with the variable references
// replaced by their values. The connection stored in the project is never
// modified, so resolved credentials cannot be written back to a file.
func Resolve(conn Connection, lookup LookupFunc) (Connection, error) {
	original := reflect.ValueOf(conn).Elem()
	resolved := reflect.New(original.Type())
	resolved.Elem().Set(original)

	for _, f := range fieldsOf(resolved.Elem()) {
		if f.value.Kind() != reflect.String {
			continue
		}
		value, err := expand(f.value.String(), lookup)
		if err != nil {
			return nil, fmt.Errorf("connection.%s: %w", f.name, err)
		}
		f.value.SetString(value)
	}
	return resolved.Interface().(Connection), nil
}
