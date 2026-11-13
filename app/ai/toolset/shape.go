/*
|--------------------------------------------------------------------------
| Smaller Results
|--------------------------------------------------------------------------
|
| A tool's result is read on every later call of the turn, so the layer
| trims it before the model sees it: only the fields the model asked
| for, at most a set number of items per list with a marker saying how
| many more there are, and long text cut with a ref that read_rest
| answers with the next part.
|
*/

package toolset

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

/*
Limits say how far results are trimmed: items kept per list and
characters kept of one text.
*/
type Limits struct {
	Items int
	Text  int
}

/*
DefaultLimits keep 20 items per list and 2,000 characters of a text.
*/
func DefaultLimits() Limits {
	return Limits{Items: 20, Text: 2000}
}

/*
keepAlways are the fields a projection keeps whether asked for or not,
so a trimmed item can still be named.
*/
func keepAlways() []string {
	return []string{"id", "name"}
}

/*
Shape trims a tool's result: the asked fields, capped lists and cut
text. A result that is not JSON is cut as text.
*/
func (s *Set) Shape(result string, fields []string) string {
	var value any
	if json.Unmarshal([]byte(result), &value) != nil {
		return s.cut(result)
	}
	value = s.trim(value, fields, 0)
	out, err := json.Marshal(value)
	if err != nil {
		return result
	}
	return string(out)
}

/*
trim walks a decoded result: lists are capped, objects projected to
the asked fields at the item level, and strings cut.
*/
func (s *Set) trim(v any, fields []string, depth int) any {
	switch t := v.(type) {
	case []any:
		kept := t
		if s.limits.Items > 0 && len(t) > s.limits.Items {
			kept = t[:s.limits.Items]
		}
		out := make([]any, 0, len(kept)+1)
		for _, item := range kept {
			out = append(out, s.trim(project(item, fields), nil, depth+1))
		}
		if len(kept) < len(t) {
			out = append(out, map[string]any{
				"more": len(t) - len(kept),
				"hint": "more results not shown; narrow the filter or ask for the next page",
			})
		}
		return out
	case map[string]any:
		if depth == 0 && len(fields) > 0 && !hasList(t) {
			t = projectObject(t, fields)
			fields = nil
		}
		for k, child := range t {
			t[k] = s.trim(child, fields, depth+1)
		}
		return t
	case string:
		return s.cut(t)
	default:
		return v
	}
}

/*
hasList reports whether an object holds a list, the items of which the
fields then apply to.
*/
func hasList(m map[string]any) bool {
	for _, v := range m {
		if _, ok := v.([]any); ok {
			return true
		}
	}
	return false
}

/*
project keeps only the asked fields of an object, and the fields that
name it.
*/
func project(v any, fields []string) any {
	m, ok := v.(map[string]any)
	if !ok || len(fields) == 0 {
		return v
	}
	return projectObject(m, fields)
}

/*
projectObject keeps only the asked fields of an object.
*/
func projectObject(m map[string]any, fields []string) map[string]any {
	out := map[string]any{}
	for k, val := range m {
		if slices.Contains(fields, k) || slices.Contains(keepAlways(), k) {
			out[k] = val
		}
	}
	return out
}

/*
cut keeps the start of a long text and stores the rest for read_rest.
*/
func (s *Set) cut(text string) string {
	if s.limits.Text <= 0 || utf8.RuneCountInString(text) <= s.limits.Text {
		return text
	}
	head, rest := splitRunes(text, s.limits.Text)
	s.mu.Lock()
	s.cuts = append(s.cuts, rest)
	ref := len(s.cuts)
	s.mu.Unlock()
	return fmt.Sprintf("%s …[cut: %d more characters; call_tool read_rest {\"ref\": %d}]", head,
		utf8.RuneCountInString(rest), ref)
}

/*
Rest answers read_rest: the next part of a cut text, itself cut again
when still long.
*/
func (s *Set) Rest(args json.RawMessage) (string, error) {
	var in struct {
		Ref json.Number `json:"ref"`
	}
	if err := decodeArgs(args, &in); err != nil {
		return "", err
	}
	n, err := strconv.Atoi(strings.TrimSpace(in.Ref.String()))
	s.mu.Lock()
	ok := err == nil && n >= 1 && n <= len(s.cuts)
	var rest string
	if ok {
		rest = s.cuts[n-1]
	}
	s.mu.Unlock()
	if !ok {
		return "", fmt.Errorf("%w: no cut text with ref %q", ErrArguments, in.Ref.String())
	}
	return s.cut(rest), nil
}

/*
splitRunes splits a text after n characters.
*/
func splitRunes(text string, n int) (string, string) {
	i := 0
	for pos := range text {
		if i == n {
			return text[:pos], text[pos:]
		}
		i++
	}
	return text, ""
}
