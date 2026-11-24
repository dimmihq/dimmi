/*
|--------------------------------------------------------------------------
| Compact Results
|--------------------------------------------------------------------------
|
| A result is read on every later call of the turn, so it is written as
| tightly as the model can still read it: a list of records becomes one
| short line per record instead of JSON objects repeating the same keys,
| empty and null fields are dropped, and long public ids become short
| aliases, c1 or p3, for the length of the run. The server maps an alias
| back to its id when the model passes it to a tool; an alias from an
| earlier, pruned turn fails with a message saying to look it up again.
|
*/

package toolset

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

/*
publicID matches a public id: a short prefix, an underscore and a
26-character body.
*/
var publicID = regexp.MustCompile(`\b([a-z]{2,8})_([0-9a-z]{26})\b`)

/*
aliasPattern matches an alias the layer hands out.
*/
var aliasPattern = regexp.MustCompile(`^[a-z]\d{1,4}$`)

/*
aliases maps public ids to short aliases and back for one run.
*/
type aliases struct {
	byID    map[string]string
	byAlias map[string]string
	count   map[byte]int
}

/*
WithCompact turns compact results on.
*/
func (s *Set) WithCompact() *Set {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.alias = &aliases{byID: map[string]string{}, byAlias: map[string]string{}, count: map[byte]int{}}
	return s
}

/*
Compact rewrites a result in compact form when compact results are on.
*/
func (s *Set) Compact(result string) string {
	if s.alias == nil {
		return result
	}
	if w, ok := wrapped(result); ok {
		return w.open + s.compactText(w.inner) + w.end
	}
	return s.compactText(result)
}

/*
compactText writes JSON as lines and shortens ids.
*/
func (s *Set) compactText(text string) string {
	var value any
	if json.Unmarshal([]byte(text), &value) == nil {
		if lines := toLines(value, ""); lines != "" {
			text = lines
		}
	}
	return s.shortenIDs(text)
}

/*
shortenIDs replaces every public id with its alias. An id inside an
address, right after a slash, is left whole: an alias there would make
a link that opens nothing.
*/
func (s *Set) shortenIDs(text string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var b strings.Builder
	last := 0
	for _, at := range publicID.FindAllStringIndex(text, -1) {
		b.WriteString(text[last:at[0]])
		last = at[1]
		id := text[at[0]:at[1]]
		if at[0] > 0 && text[at[0]-1] == '/' {
			b.WriteString(id)
			continue
		}
		b.WriteString(s.aliasOf(id))
	}
	b.WriteString(text[last:])
	return b.String()
}

/*
aliasOf is the alias of an id, handed out the first time it is seen.
*/
func (s *Set) aliasOf(id string) string {
	if a, ok := s.alias.byID[id]; ok {
		return a
	}
	letter := id[0]
	s.alias.count[letter]++
	a := string(letter) + strconv.Itoa(s.alias.count[letter])
	s.alias.byID[id], s.alias.byAlias[a] = a, id
	return a
}

/*
Expand maps the aliases in a tool call's arguments back to their ids.
An alias the run never handed out is refused with a message the model
can act on.
*/
func (s *Set) Expand(args json.RawMessage) (json.RawMessage, error) {
	if s.alias == nil || len(args) == 0 {
		return args, nil
	}
	var value any
	if !decodes(args, &value) {
		return args, nil
	}
	var unknown string
	value = s.replaceAliases(value, "", &unknown)
	if unknown != "" {
		return nil, fmt.Errorf("%w: %s is a short id from an earlier answer; look the record up again to get "+
			"its current id", ErrArguments, unknown)
	}
	if out, ok := encodes(value); ok {
		return out, nil
	}
	return args, nil
}

/*
decodes reports whether arguments decode as JSON.
*/
func decodes(args json.RawMessage, v *any) bool {
	return json.Unmarshal(args, v) == nil
}

/*
encodes writes a value as JSON, reporting whether it could.
*/
func encodes(v any) (json.RawMessage, bool) {
	out, err := json.Marshal(v)
	return out, err == nil
}

/*
replaceAliases walks the arguments, replacing aliases.
*/
func (s *Set) replaceAliases(v any, key string, unknown *string) any {
	switch t := v.(type) {
	case map[string]any:
		for k, child := range t {
			t[k] = s.replaceAliases(child, k, unknown)
		}
		return t
	case []any:
		for i, child := range t {
			t[i] = s.replaceAliases(child, key, unknown)
		}
		return t
	case string:
		if !aliasPattern.MatchString(t) {
			return t
		}
		s.mu.Lock()
		id, ok := s.alias.byAlias[t]
		handed := s.alias.count[t[0]] > 0
		s.mu.Unlock()
		if ok {
			return id
		}
		if idKey(key) || handed {
			*unknown = t
		}
		return t
	default:
		return v
	}
}

/*
idKey reports whether an argument names an id.
*/
func idKey(key string) bool {
	k := strings.ToLower(key)
	return k == "id" || strings.HasSuffix(k, "_id") || strings.HasSuffix(k, "_ids") || k == "ids"
}

/*
toLines writes a decoded value as short lines: a record as key: value
pairs on one line, a list one record per line, nested lists indented.
It returns "" for a bare scalar, which reads well as it is.
*/
func toLines(v any, indent string) string {
	switch t := v.(type) {
	case []any:
		var b strings.Builder
		for _, item := range t {
			b.WriteString(indent + "- " + strings.TrimLeft(line(item, indent+"  "), " ") + "\n")
		}
		return strings.TrimRight(b.String(), "\n")
	case map[string]any:
		return line(t, indent)
	default:
		return ""
	}
}

/*
line writes one record: scalar fields inline, list and object fields on
their own indented lines.
*/
func line(v any, indent string) string {
	m, ok := v.(map[string]any)
	if !ok {
		return fmt.Sprint(scalar(v))
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var inline, nested []string
	for _, k := range keys {
		in, nest := field(k, m[k], indent)
		inline, nested = appendNonEmpty(inline, in), appendNonEmpty(nested, nest)
	}
	out := indent + strings.Join(inline, "; ")
	if len(nested) > 0 {
		out += "\n" + strings.Join(nested, "\n")
	}
	return out
}

/*
field writes one field, inline or nested; empty and null fields write
nothing.
*/
func field(k string, v any, indent string) (string, string) {
	switch child := v.(type) {
	case nil:
		return "", ""
	case string:
		if child == "" {
			return "", ""
		}
		return k + ": " + child, ""
	case []any:
		if len(child) == 0 {
			return "", ""
		}
		return "", indent + k + ":\n" + toLines(child, indent+"  ")
	case map[string]any:
		if len(child) == 0 {
			return "", ""
		}
		return "", indent + k + ": " + line(child, indent+"  ")
	default:
		return k + ": " + fmt.Sprint(scalar(child)), ""
	}
}

/*
appendNonEmpty appends a text unless it is empty.
*/
func appendNonEmpty(list []string, text string) []string {
	if text == "" {
		return list
	}
	return append(list, text)
}

/*
scalar formats a number without a needless exponent.
*/
func scalar(v any) any {
	if f, ok := v.(float64); ok && f == float64(int64(f)) {
		return int64(f)
	}
	return v
}
