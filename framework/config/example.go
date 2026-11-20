/*
|--------------------------------------------------------------------------
| Example Configuration
|--------------------------------------------------------------------------
|
| An application's example file lists every setting it reads, so nobody
| has to read the structs to learn what can be set. The file is written
| from the structs themselves: each field under its key at its default,
| with the environment variable that sets the same value beside it, and
| each section under what its struct's comment says of it. Values the
| example already sets, such as a local database address, are kept, so
| the file stays a working configuration; a key no struct reads is an
| error, which catches a setting that was renamed or misspelled.
|
| Sections whose options depend on a driver, such as a mailer's, are
| raw branches: the structs cannot list their options, so the example
| shows only the entries it was given.
|
*/

package config

import (
	"encoding"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/knadh/koanf/parsers/yaml"
)

/*
ErrUnknownExampleKey is wrapped when the example sets a key no
configuration struct reads.
*/
var ErrUnknownExampleKey = errors.New("config: example sets a key nothing reads")

/*
ExampleOptions says how the example is written. Header opens the file.
EnvPrefix is the prefix of the environment variables, such as APP.
Path is the example as it stands, whose values are kept; a file that
does not exist yet sets none. Module is the module's import path and
Root the directory its source is in, for reading what each struct's
comment says; without them sections carry no comment.
*/
type ExampleOptions struct {
	Header    string
	EnvPrefix string
	Path      string
	Module    string
	Root      string
}

/*
Example writes the example file for the configuration structs the
targets point at, merged under their shared keys.
*/
func Example(opts ExampleOptions, targets ...any) ([]byte, error) {
	current, err := currentExample(opts.Path)
	if err != nil {
		return nil, err
	}

	w := &exampleWriter{docs: exampleDocs{module: opts.Module, root: opts.Root, read: map[string]map[string]string{}}}
	root := &exampleNode{env: strings.TrimSuffix(opts.EnvPrefix, "_")}

	for _, target := range targets {
		if err := applyDefaults(target); err != nil {
			return nil, fmt.Errorf("apply defaults: %w", err)
		}

		w.fields(root, reflect.ValueOf(target).Elem(), current)
	}

	if unknown := unread(root, current, ""); len(unknown) > 0 {
		return nil, fmt.Errorf("%w: %s", ErrUnknownExampleKey, strings.Join(unknown, ", "))
	}

	var out strings.Builder

	if header := strings.TrimSpace(opts.Header); header != "" {
		for line := range strings.SplitSeq(header, "\n") {
			out.WriteString(strings.TrimRight("# "+line, " ") + "\n")
		}
	}

	for _, section := range root.children {
		out.WriteString("\n")
		section.write(&out, 0)
	}

	return []byte(out.String()), nil
}

/*
exampleNode is one key of the example: a section with children, or a
setting with its value. Env is the variable that sets the key, shown
beside a setting and passed down by a section.
*/
type exampleNode struct {
	key      string
	doc      string
	value    string
	env      string
	note     string
	setting  bool
	children []*exampleNode
}

func (n *exampleNode) child(key string) *exampleNode {
	for _, c := range n.children {
		if c.key == key {
			return c
		}
	}

	c := &exampleNode{key: key, env: n.env + levelSeparator + strings.ToUpper(key)}
	if n.key == "" {
		c.env = n.env + "_" + strings.ToUpper(key)
	}

	n.children = append(n.children, c)

	return c
}

func (n *exampleNode) set(value string) {
	n.value, n.setting = value, true
}

func (n *exampleNode) write(out *strings.Builder, depth int) {
	pad := strings.Repeat("  ", depth)

	if n.doc != "" {
		for line := range strings.SplitSeq(strings.TrimSpace(n.doc), "\n") {
			out.WriteString(strings.TrimRight(pad+"# "+line, " ") + "\n")
		}
	}

	line := pad + n.key + ":"
	if n.setting {
		line += " " + n.value + "  # " + n.env + n.note
	}

	out.WriteString(line + "\n")

	for _, c := range n.children {
		c.write(out, depth+1)
	}
}

/*
exampleWriter builds the example's keys from struct fields.
*/
type exampleWriter struct {
	docs exampleDocs
}

/*
fields adds a struct's settings under the node. Current holds the values
the example sets for this struct's keys.
*/
func (w *exampleWriter) fields(parent *exampleNode, v reflect.Value, current map[string]any) {
	t := v.Type()

	for i := range t.NumField() {
		field := t.Field(i)

		key := field.Tag.Get("koanf")
		if !field.IsExported() || key == "" || key == "-" {
			continue
		}

		node := parent.child(key)
		if field.Tag.Get("secret") == "true" {
			node.note = " (secret; or " + node.env + secretSuffix + " naming a file)"
		}

		w.field(node, v.Field(i), current[key])
	}
}

/*
field fills one key: a struct becomes a section, a map its entries, and
anything else a setting. A section that is absent unless configured
stays absent.
*/
func (w *exampleWriter) field(node *exampleNode, v reflect.Value, current any) {
	if v.Kind() == reflect.Pointer && v.Type().Elem().Kind() == reflect.Struct {
		if v.IsNil() && current == nil {
			node.set("null")

			return
		}

		v = filled(v)
	}

	switch {
	case v.Kind() == reflect.Struct && !isText(v):
		w.section(node, v, current)
	case v.Kind() == reflect.Map:
		w.entries(node, v, current)
	case current != nil:
		w.raw(node, current)
	default:
		node.set(scalar(v))
	}
}

/*
filled returns the struct a pointer points at with its defaults, a new
one when the pointer is nil.
*/
func filled(v reflect.Value) reflect.Value {
	out := reflect.New(v.Type().Elem())
	if !v.IsNil() {
		out.Elem().Set(v.Elem())
	}

	_ = defaultStruct(out.Elem())

	return out.Elem()
}

func (w *exampleWriter) section(node *exampleNode, v reflect.Value, current any) {
	if node.doc == "" {
		node.doc = w.docs.of(v.Type())
	}

	nested, _ := current.(map[string]any)
	w.fields(node, v, nested)
}

/*
entries fills a map: each entry the example sets, as a section when the
map holds structs and as given otherwise. A map with no entries says how
an entry's variable is named.
*/
func (w *exampleWriter) entries(node *exampleNode, v reflect.Value, current any) {
	given, _ := current.(map[string]any)
	if len(given) == 0 {
		node.set("{}")
		node.note = levelSeparator + "<NAME>"

		if kind := v.Type().Elem().Kind(); kind == reflect.Map || kind == reflect.Struct {
			node.note += levelSeparator + "<OPTION>"
		}

		return
	}

	elem := v.Type().Elem()

	for _, name := range sortedKeys(given) {
		entry := node.child(name)

		if elem.Kind() != reflect.Struct {
			w.raw(entry, given[name])

			continue
		}

		value := reflect.New(elem)
		_ = defaultStruct(value.Elem())
		w.section(entry, value.Elem(), given[name])
	}
}

/*
raw writes a value the example sets as it was given: a branch as nested
keys, anything else as a setting.
*/
func (w *exampleWriter) raw(node *exampleNode, current any) {
	branch, ok := current.(map[string]any)
	if !ok {
		node.set(plain(current))

		return
	}

	for _, key := range sortedKeys(branch) {
		w.raw(node.child(key), branch[key])
	}
}

/*
unread lists the keys the example sets that no node was written for.
*/
func unread(node *exampleNode, current map[string]any, path string) []string {
	var out []string

	for _, key := range sortedKeys(current) {
		at := strings.TrimPrefix(path+"."+key, ".")

		i := slices.IndexFunc(node.children, func(c *exampleNode) bool { return c.key == key })
		if i < 0 {
			out = append(out, at)

			continue
		}

		if nested, ok := current[key].(map[string]any); ok && len(node.children[i].children) > 0 {
			out = append(out, unread(node.children[i], nested, at)...)
		}
	}

	return out
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return keys
}

func isText(v reflect.Value) bool {
	_, ok := reflect.TypeAssert[encoding.TextMarshaler](v)

	return ok || v.Type() == reflect.TypeFor[time.Time]()
}

/*
scalar writes a field's value the way YAML reads it back.
*/
func scalar(v reflect.Value) string {
	if v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return "null"
		}

		return scalar(v.Elem())
	}

	if d, ok := reflect.TypeAssert[time.Duration](v); ok {
		return duration(d)
	}

	if v.Kind() == reflect.Slice {
		items := make([]any, 0, v.Len())
		for i := range v.Len() {
			items = append(items, v.Index(i).Interface())
		}

		return plain(items)
	}

	return plain(v.Interface())
}

/*
duration writes a duration without the zero units Go prints: 5m, not
5m0s.
*/
func duration(d time.Duration) string {
	text := d.String()
	if strings.HasSuffix(text, "m0s") {
		text = strings.TrimSuffix(text, "0s")
	}

	if strings.HasSuffix(text, "h0m") {
		text = strings.TrimSuffix(text, "0m")
	}

	return text
}

/*
plain writes a value as a YAML scalar or a list on one line.
*/
func plain(value any) string {
	switch typed := value.(type) {
	case nil:
		return `""`
	case []any:
		items := make([]string, 0, len(typed))
		for _, item := range typed {
			items = append(items, plain(item))
		}

		return "[" + strings.Join(items, ", ") + "]"
	case encoding.TextMarshaler:
		text, _ := typed.MarshalText()

		return quoted(string(text))
	}

	v := reflect.ValueOf(value)
	if v.Kind() == reflect.String {
		return quoted(v.String())
	}

	if v.Kind() == reflect.Slice {
		return scalar(v)
	}

	return fmt.Sprint(value)
}

/*
quoted writes a string bare when YAML reads it back as the same string,
and in quotes otherwise.
*/
func quoted(text string) string {
	if text == "" {
		return `""`
	}

	parsed, err := yaml.Parser().Unmarshal([]byte("v: " + text))
	if back, ok := parsed["v"].(string); err == nil && ok && back == text && !strings.Contains(text, " #") {
		return text
	}

	return strconv.Quote(text)
}
