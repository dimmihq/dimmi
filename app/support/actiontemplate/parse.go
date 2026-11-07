/*
|--------------------------------------------------------------------------
| Placeholder Grammar
|--------------------------------------------------------------------------
|
| One placeholder grammar everywhere a request is described: {ns.path},
| where the namespace is input, contact or connection and the path is
| dotted lowercase words. Text in braces that does not match stays
| literal text. There is no namespace for secrets: a credential only
| ever enters a request through the connection's auth, after rendering,
| so a template can never leak one.
|
*/

package actiontemplate

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

/*
The namespaces a placeholder may name.
*/
const (
	NamespaceInput      = "input"
	NamespaceContact    = "contact"
	NamespaceConnection = "connection"
)

/*
The contact values an action may bind to. Attr reads one custom
attribute, as contact.attr.<key>.
*/
const (
	ContactEmail      = "email"
	ContactExternalID = "external_id"
	ContactAttr       = "attr"
)

/*
Template errors. ErrUnknownVariable names a placeholder no value can
fill; ErrMissing is a required value the call does not have;
ErrInvalidPathValue a path segment that would climb the path;
ErrInvalidHeader a header value carrying a line break; ErrTypeMismatch an
object or list placed inside a string.
*/
var (
	ErrUnknownVariable  = errors.New("unknown template variable")
	ErrMissing          = errors.New("template value missing")
	ErrInvalidPathValue = errors.New("invalid path value")
	ErrInvalidHeader    = errors.New("invalid header value")
	ErrTypeMismatch     = errors.New("template type mismatch")
	ErrInvalidTemplate  = errors.New("invalid template")
)

/*
Ref is one placeholder: its namespace and the dotted path inside it.
*/
type Ref struct {
	Namespace string
	Path      []string
}

/*
String writes the placeholder back as it appears in a template.
*/
func (r Ref) String() string {
	return "{" + r.Namespace + "." + strings.Join(r.Path, ".") + "}"
}

/*
UsesContact reports whether the placeholder binds the customer's
identity.
*/
func (r Ref) UsesContact() bool {
	return r.Namespace == NamespaceContact
}

/*
placeholder matches one placeholder of the grammar.
*/
func placeholder() *regexp.Regexp {
	return regexp.MustCompile(`\{(input|contact|connection)\.([A-Za-z0-9_]+(?:\.[A-Za-z0-9_]+)*)\}`)
}

/*
exact matches a string that is exactly one placeholder.
*/
func exact() *regexp.Regexp {
	return regexp.MustCompile(`^\{(input|contact|connection)\.([A-Za-z0-9_]+(?:\.[A-Za-z0-9_]+)*)\}$`)
}

/*
Parse returns the placeholders a text holds, in order.
*/
func Parse(text string) []Ref {
	var out []Ref
	for _, m := range placeholder().FindAllStringSubmatch(text, -1) {
		out = append(out, Ref{Namespace: m[1], Path: strings.Split(m[2], ".")})
	}
	return out
}

/*
Refs returns every placeholder inside a decoded JSON value: in its
strings, never its keys.
*/
func Refs(v any) []Ref {
	switch t := v.(type) {
	case string:
		return Parse(t)
	case map[string]any:
		var out []Ref
		for _, k := range sortedKeys(t) {
			out = append(out, Refs(t[k])...)
		}
		return out
	case []any:
		var out []Ref
		for _, item := range t {
			out = append(out, Refs(item)...)
		}
		return out
	}
	return nil
}

/*
Check validates a placeholder against what can fill it: an input
property the schema declares, an identity value the contact can carry,
or a variable the connection holds.
*/
func Check(r Ref, inputs, variables []string) error {
	switch r.Namespace {
	case NamespaceInput:
		if !slices.Contains(inputs, r.Path[0]) {
			return fmt.Errorf("%w: %s is not an input property", ErrUnknownVariable, r)
		}
	case NamespaceContact:
		if !contactPath(r.Path) {
			return fmt.Errorf("%w: %s; use contact.email, contact.external_id or contact.attr.<key>",
				ErrUnknownVariable, r)
		}
	case NamespaceConnection:
		if len(r.Path) != 1 || !slices.Contains(variables, r.Path[0]) {
			return fmt.Errorf("%w: %s is not a connection variable", ErrUnknownVariable, r)
		}
	}
	return nil
}

/*
contactPath reports whether a contact placeholder names a value an
action may bind to.
*/
func contactPath(path []string) bool {
	switch {
	case len(path) == 1:
		return path[0] == ContactEmail || path[0] == ContactExternalID
	case len(path) == 2:
		return path[0] == ContactAttr
	}
	return false
}

/*
CheckBraces refuses a template whose braces hold anything but
placeholders of the grammar, such as URI template operators.
*/
func CheckBraces(text string) error {
	rest := placeholder().ReplaceAllString(text, "")
	if strings.ContainsAny(rest, "{}") {
		return fmt.Errorf("%w: braces may only hold {input.x}, {contact.x} or {connection.x}", ErrInvalidTemplate)
	}
	return nil
}

/*
sortedKeys lists a map's keys in order, so walks are deterministic.
*/
func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
