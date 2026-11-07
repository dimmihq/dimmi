/*
|--------------------------------------------------------------------------
| Template Expansion
|--------------------------------------------------------------------------
|
| A path placeholder is always one escaped segment, so a value can never
| add a slash or climb with "." or "..". A string with placeholders is
| plain interpolation of scalar values; a JSON body keeps types, so a
| number the model gave stays a number and a missing optional key is
| left out rather than sent empty. Header values carrying line breaks
| are refused, so a value can never smuggle in a header of its own.
|
*/

package actiontemplate

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/yosida95/uritemplate/v3"
)

/*
maxHeaderValue caps one rendered header value in bytes.
*/
const maxHeaderValue = 1024

/*
Vars are the values a call's templates are filled with: the validated
input, the bound contact values (email, external_id and attr, a map of
custom attributes) and the connection's variables.
*/
type Vars struct {
	Input      map[string]any
	Contact    map[string]any
	Connection map[string]string
}

/*
Lookup returns the value a placeholder names, and whether there is one.
*/
func (v Vars) Lookup(r Ref) (any, bool) {
	switch r.Namespace {
	case NamespaceInput:
		return walk(v.Input, r.Path)
	case NamespaceContact:
		return walk(v.Contact, r.Path)
	case NamespaceConnection:
		value, ok := v.Connection[strings.Join(r.Path, ".")]
		return value, ok && value != ""
	}
	return nil, false
}

/*
walk follows a dotted path into nested objects.
*/
func walk(root map[string]any, path []string) (any, bool) {
	var cur any = root
	for _, key := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		if cur, ok = m[key]; !ok || cur == nil {
			return nil, false
		}
	}
	return cur, true
}

/*
ExpandPath fills a path template, each value escaped as exactly one
segment.
*/
func ExpandPath(tpl string, v Vars) (string, error) {
	if err := CheckBraces(tpl); err != nil {
		return "", err
	}
	values := uritemplate.Values{}
	for _, r := range Parse(tpl) {
		raw, ok := v.Lookup(r)
		if !ok {
			return "", fmt.Errorf("%w: %s", ErrMissing, r)
		}
		text, err := scalar(raw)
		if err != nil {
			return "", fmt.Errorf("%s: %w", r, err)
		}
		if text == "" || text == "." || text == ".." {
			return "", fmt.Errorf("%w: %s", ErrInvalidPathValue, r)
		}
		values[r.Namespace+"."+strings.Join(r.Path, ".")] = uritemplate.String(text)
	}
	t, err := uritemplate.New(tpl)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrInvalidTemplate, err)
	}
	out, err := t.Expand(values)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrInvalidTemplate, err)
	}
	return out, nil
}

/*
ExpandString interpolates a text. A text that is one placeholder with
no value reports false, so the caller leaves the field out; a missing
value inside longer text is an error.
*/
func ExpandString(tpl string, v Vars) (string, bool, error) {
	if m := exact().FindStringSubmatch(tpl); m != nil {
		raw, ok := v.Lookup(Ref{Namespace: m[1], Path: strings.Split(m[2], ".")})
		if !ok {
			return "", false, nil
		}
		text, err := scalar(raw)
		return text, err == nil, err
	}

	var failure error
	out := placeholder().ReplaceAllStringFunc(tpl, func(match string) string {
		m := placeholder().FindStringSubmatch(match)
		r := Ref{Namespace: m[1], Path: strings.Split(m[2], ".")}
		raw, ok := v.Lookup(r)
		if !ok {
			failure = fmt.Errorf("%w: %s", ErrMissing, r)
			return ""
		}
		text, err := scalar(raw)
		if err != nil {
			failure = fmt.Errorf("%s: %w", r, err)
		}
		return text
	})
	return out, true, failure
}

/*
ExpandHeader interpolates a header value and refuses one that could
split the header.
*/
func ExpandHeader(tpl string, v Vars) (string, bool, error) {
	out, ok, err := ExpandString(tpl, v)
	if err != nil || !ok {
		return "", ok, err
	}
	if err := CheckHeaderValue(out); err != nil {
		return "", false, err
	}
	return out, true, nil
}

/*
CheckHeaderValue refuses a header value with a line break or a NUL, or
longer than a header value may be.
*/
func CheckHeaderValue(value string) error {
	if strings.ContainsAny(value, "\r\n\x00") || len(value) > maxHeaderValue {
		return ErrInvalidHeader
	}
	return nil
}

/*
ExpandJSON fills a decoded JSON value, keeping types: a string that is
exactly one placeholder becomes the value itself, a missing one drops
its key, and any other string is interpolated. It reports false when
the value itself is dropped.
*/
func ExpandJSON(tpl any, v Vars) (any, bool, error) {
	switch t := tpl.(type) {
	case string:
		if m := exact().FindStringSubmatch(t); m != nil {
			raw, ok := v.Lookup(Ref{Namespace: m[1], Path: strings.Split(m[2], ".")})
			return raw, ok, nil
		}
		out, _, err := ExpandString(t, v)
		return out, true, err
	case map[string]any:
		out, err := expandObject(t, v)
		return out, err == nil, err
	case []any:
		out, err := expandList(t, v)
		return out, err == nil, err
	}
	return tpl, true, nil
}

/*
expandObject fills every value of an object, dropping the keys left
without one.
*/
func expandObject(tpl map[string]any, v Vars) (map[string]any, error) {
	out := make(map[string]any, len(tpl))
	for _, k := range sortedKeys(tpl) {
		value, ok, err := ExpandJSON(tpl[k], v)
		if err != nil {
			return nil, err
		}
		if ok {
			out[k] = value
		}
	}
	return out, nil
}

/*
expandList fills every item of a list, dropping the items left without
a value.
*/
func expandList(tpl []any, v Vars) ([]any, error) {
	out := make([]any, 0, len(tpl))
	for _, item := range tpl {
		value, ok, err := ExpandJSON(item, v)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, value)
		}
	}
	return out, nil
}

/*
ExpandForm fills a flat object of form fields; keys are literal, values
interpolated, and a field with no value is left out.
*/
func ExpandForm(tpl map[string]any, v Vars) (url.Values, error) {
	out := url.Values{}
	for _, k := range sortedKeys(tpl) {
		text, isText := tpl[k].(string)
		if !isText {
			value, err := scalar(tpl[k])
			if err != nil {
				return nil, fmt.Errorf("form field %s: %w", k, err)
			}
			out.Set(k, value)
			continue
		}
		value, ok, err := ExpandString(text, v)
		if err != nil {
			return nil, fmt.Errorf("form field %s: %w", k, err)
		}
		if ok {
			out.Set(k, value)
		}
	}
	return out, nil
}

/*
scalar writes a value as text; objects and lists cannot sit inside a
string.
*/
func scalar(v any) (string, error) {
	switch t := v.(type) {
	case string:
		return t, nil
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64), nil
	case int:
		return strconv.Itoa(t), nil
	case int64:
		return strconv.FormatInt(t, 10), nil
	case bool:
		return strconv.FormatBool(t), nil
	case nil:
		return "", nil
	}
	return "", ErrTypeMismatch
}
