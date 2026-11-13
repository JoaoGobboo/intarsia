package project

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/token"
)

// rewrite updates the text of a YAML file after its content changed from
// before to after, two versions of the same Go value.
//
// YAML libraries reformat the whole file when they write it back: they realign
// comments and normalise spacing. Instead, rewrite finds what differs between
// before and after and edits only those lines of the original text. Whatever
// did not change is untouched, so a file saved without changes is identical
// byte for byte, including fields this program does not know about.
//
// before is nil when the original text could not be read into a value; then
// every field of after is written and nothing is removed.
func rewrite(original []byte, before, after any) ([]byte, error) {
	was, err := toTree(before)
	if err != nil {
		return nil, err
	}
	now, err := toTree(after)
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(original))) == 0 {
		return marshal(now)
	}

	parsed, err := parseYAML(original)
	if err != nil {
		return nil, fmt.Errorf("the file is not valid YAML; fix it by hand before saving over it: %w", err)
	}
	if len(parsed.Docs) != 1 {
		return nil, errors.New("the file has more than one YAML document")
	}

	r := newRewriter(original)
	switch root := parsed.Docs[0].Body.(type) {
	case *ast.MappingNode:
		if root.IsFlowStyle || len(root.Values) == 0 {
			return nil, errors.New("the file is written as {...} at the top level; use one field per line")
		}
		r.patch(root, was, now, len(r.lines))
	case nil, *ast.CommentGroupNode:
		// Only comments: keep them and write the content below.
		for _, item := range now {
			r.insert(len(r.lines), renderEntry(keyOf(item), item.Value), "")
		}
	default:
		return nil, errors.New("the file does not contain fields in the form `name: value`")
	}
	return r.apply(original), nil
}

// toTree converts a value to the ordered, generic form of its YAML encoding,
// so that two versions can be compared field by field.
func toTree(v any) (yaml.MapSlice, error) {
	if v == nil {
		return nil, nil
	}
	data, err := marshal(v)
	if err != nil {
		return nil, err
	}
	var tree yaml.MapSlice
	if err := yaml.UnmarshalWithOptions(data, &tree, yaml.UseOrderedMap()); err != nil {
		return nil, err
	}
	return tree, nil
}

func keyOf(item yaml.MapItem) string { return fmt.Sprint(item.Key) }

func valueOf(tree yaml.MapSlice, key string) (any, bool) {
	for _, item := range tree {
		if keyOf(item) == key {
			return item.Value, true
		}
	}
	return nil, false
}

// renderEntry returns the lines of `key: value`, without indentation.
func renderEntry(key string, value any) []string {
	data, err := marshal(yaml.MapSlice{{Key: key, Value: value}})
	if err != nil {
		// The value came out of toTree, so it has already been encoded once.
		panic(fmt.Sprintf("project: cannot encode %q: %v", key, err))
	}
	return strings.Split(strings.TrimRight(string(data), "\n"), "\n")
}

// edit replaces lines[start:end] with new lines. An insertion has start == end.
type edit struct {
	start, end int
	lines      []string
}

type rewriter struct {
	lines []string // the original text, one line each, line endings kept
	eol   string
	edits []edit
}

func newRewriter(original []byte) *rewriter {
	text := string(original)
	r := &rewriter{lines: strings.SplitAfter(text, "\n"), eol: "\n"}
	if last := len(r.lines) - 1; r.lines[last] == "" {
		r.lines = r.lines[:last]
	}
	if strings.Contains(text, "\r\n") {
		r.eol = "\r\n"
	}
	return r
}

// entry is one `key: value` of a mapping and the lines it occupies.
type entry struct {
	key         string
	node        *ast.MappingValueNode
	first, last int // line indexes, inclusive
}

// lineOf is the index in rewriter.lines of the line where a token starts.
func lineOf(tok *token.Token) int { return tok.Position.Line - 1 }

// entries locates the entries of a mapping whose text ends before line end.
func (r *rewriter) entries(mapping *ast.MappingNode, end int) []entry {
	entries := make([]entry, len(mapping.Values))
	for i, node := range mapping.Values {
		key := node.Key.GetToken()
		limit := end
		if i+1 < len(mapping.Values) {
			limit = lineOf(mapping.Values[i+1].Key.GetToken())
		}
		// Blank lines and comments after the value belong to whatever follows.
		last := limit - 1
		for last > lineOf(key) && r.isOutside(last, key.Position.Column-1) {
			last--
		}
		entries[i] = entry{key: key.Value, node: node, first: lineOf(key), last: last}
	}
	return entries
}

// isOutside reports whether a line is blank, or a comment that is not indented
// deeper than indent: such a line is not part of an entry at that indentation.
func (r *rewriter) isOutside(line, indent int) bool {
	text := strings.TrimRight(r.lines[line], "\r\n")
	content := strings.TrimLeft(text, " ")
	if strings.TrimSpace(content) == "" {
		return true
	}
	return strings.HasPrefix(content, "#") && len(text)-len(content) <= indent
}

// patch applies the difference between was and now to a block mapping.
func (r *rewriter) patch(mapping *ast.MappingNode, was, now yaml.MapSlice, end int) {
	entries := r.entries(mapping, end)
	find := func(key string) *entry {
		i := slices.IndexFunc(entries, func(e entry) bool { return e.key == key })
		if i < 0 {
			return nil
		}
		return &entries[i]
	}
	keyColumn := mapping.Values[0].Key.GetToken().Position.Column - 1
	indent := strings.Repeat(" ", keyColumn)

	set := func(key string, old, value any) {
		e := find(key)
		if e == nil {
			// After the closest field that comes before it in the new version,
			// or at the top when none does, so that new fields land in their
			// usual order.
			at := entries[0].first
			for _, item := range now {
				if keyOf(item) == key {
					break
				}
				if previous := find(keyOf(item)); previous != nil {
					at = previous.last + 1
				}
			}
			r.insert(at, renderEntry(key, value), indent)
			return
		}
		oldFields, wasMapping := old.(yaml.MapSlice)
		newFields, isMapping := value.(yaml.MapSlice)
		if nested, ok := e.node.Value.(*ast.MappingNode); ok && wasMapping && isMapping &&
			!nested.IsFlowStyle && len(nested.Values) > 0 && len(newFields) > 0 {
			r.patch(nested, oldFields, newFields, e.last+1)
			return
		}
		if r.replaceScalar(e, value) {
			return
		}
		r.replaceEntry(e, renderEntry(key, value), indent)
	}

	for _, item := range was {
		key := keyOf(item)
		value, kept := valueOf(now, key)
		switch {
		case !kept:
			if e := find(key); e != nil {
				r.remove(entries, e, keyColumn)
			}
		case !reflect.DeepEqual(item.Value, value):
			set(key, item.Value, value)
		}
	}
	for _, item := range now {
		if _, existed := valueOf(was, keyOf(item)); !existed {
			set(keyOf(item), nil, item.Value)
		}
	}
}

// replaceScalar swaps a one-line value in place, keeping the spacing and the
// comment that follow it. It reports false when the old or the new value is
// not a one-line scalar.
func (r *rewriter) replaceScalar(e *entry, value any) bool {
	switch value.(type) {
	case yaml.MapSlice, []any:
		return false
	}
	data, err := marshal(value)
	rendered := strings.TrimRight(string(data), "\n")
	if err != nil || strings.Contains(rendered, "\n") {
		return false
	}

	tok := e.node.Value.GetToken()
	if _, scalar := e.node.Value.(ast.ScalarNode); !scalar || tok == nil || e.first != e.last || lineOf(tok) != e.first {
		return false
	}
	line := r.lines[e.first]
	text := strings.TrimRight(line, "\r\n")
	start := byteOffset(text, tok.Position.Column-1)
	stop := len(text)
	if next := tok.Next; next != nil && next.Type == token.CommentType && next.Position.Line == tok.Position.Line {
		stop = byteOffset(text, next.Position.Column-1)
	}
	if start >= stop {
		return false
	}
	valueEnd := start + len(strings.TrimRight(text[start:stop], " \t"))
	if valueEnd == start {
		return false
	}
	r.edits = append(r.edits, edit{start: e.first, end: e.first + 1, lines: []string{line[:start] + rendered + line[valueEnd:]}})
	return true
}

// replaceEntry rewrites a whole entry: a list, a group of fields or a value
// that spans lines.
//
// ponytail: a changed list is rewritten whole, so comments between its items
// are lost. Diff the items one by one if that starts to matter.
func (r *rewriter) replaceEntry(e *entry, lines []string, indent string) {
	// `key:` on its own line, with the value below: keep that line, and any
	// comment on it, and replace only what is below.
	valueBelow := e.node.Value.GetToken() != nil && lineOf(e.node.Value.GetToken()) > e.first
	if valueBelow && len(lines) > 1 && strings.HasSuffix(lines[0], ":") {
		r.replace(e.first+1, e.last+1, lines[1:], indent)
		return
	}
	r.replace(e.first, e.last+1, lines, indent)
}

// remove deletes an entry together with the comment lines right above it.
func (r *rewriter) remove(entries []entry, e *entry, keyColumn int) {
	start := e.first
	i := slices.IndexFunc(entries, func(other entry) bool { return other.key == e.key })
	// The comment above the first entry usually describes the file or the
	// group, not that entry, so it stays.
	if i > 0 {
		for start-1 > entries[i-1].last && r.isCommentAt(start-1, keyColumn) {
			start--
		}
	}
	r.replace(start, e.last+1, nil, "")
}

func (r *rewriter) isCommentAt(line, indent int) bool {
	text := strings.TrimRight(r.lines[line], "\r\n")
	content := strings.TrimLeft(text, " ")
	return strings.HasPrefix(content, "#") && len(text)-len(content) == indent
}

func (r *rewriter) replace(start, end int, lines []string, indent string) {
	r.edits = append(r.edits, edit{start: start, end: end, lines: r.format(lines, indent)})
}

func (r *rewriter) insert(at int, lines []string, indent string) {
	lines = r.format(lines, indent)
	for i := range r.edits {
		// Several fields added at the same place stay in the order they came.
		if r.edits[i].start == at && r.edits[i].end == at {
			r.edits[i].lines = append(r.edits[i].lines, lines...)
			return
		}
	}
	r.edits = append(r.edits, edit{start: at, end: at, lines: lines})
}

func (r *rewriter) format(lines []string, indent string) []string {
	formatted := make([]string, len(lines))
	for i, line := range lines {
		formatted[i] = indent + line + r.eol
	}
	return formatted
}

func (r *rewriter) apply(original []byte) []byte {
	if len(r.edits) == 0 {
		return original
	}
	lines := slices.Clone(r.lines)
	if last := len(lines) - 1; !strings.HasSuffix(lines[last], "\n") {
		lines[last] += r.eol
	}
	// From the bottom up, so earlier line numbers stay valid. Where a
	// replacement and an insertion start on the same line, the replacement
	// goes first and the inserted lines end up above it.
	slices.SortStableFunc(r.edits, func(a, b edit) int {
		if a.start != b.start {
			return b.start - a.start
		}
		return (b.end - b.start) - (a.end - a.start)
	})
	for _, e := range r.edits {
		lines = slices.Concat(lines[:e.start], e.lines, lines[e.end:])
	}
	return []byte(strings.Join(lines, ""))
}

// byteOffset converts a count of characters from the start of s to an index.
func byteOffset(s string, chars int) int {
	for i := range s {
		if chars == 0 {
			return i
		}
		chars--
	}
	return len(s)
}
