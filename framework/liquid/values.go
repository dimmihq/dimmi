/*
|--------------------------------------------------------------------------
| Values
|--------------------------------------------------------------------------
|
| A template reads plain values only: nil, booleans, numbers, strings,
| HTML, times, lists and maps with string keys. Value turns what the
| server hands over into these, structs included, reading only exported
| fields and never calling a method, so a template can never run code
| that hides behind a value. The rest of the file says how values read:
| what is true, what is empty, how two values compare and how a value
| prints.
|
*/

package liquid

import (
	"fmt"
	"html"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

/*
HTML is markup a template prints as it is. Only the server's own values
are HTML; everything else is escaped when printed.
*/
type HTML string

/*
Value converts a Go value into the values templates read. A struct
becomes a map of its exported fields, each named by its liquid tag or
else by its name in snake case; a field tagged "-" is left out.
Pointers are followed, lists and maps converted all the way down, and
anything else, such as a function or a channel, reads as nil.
*/
func Value(v any) any {
	return convert(reflect.ValueOf(v), 0)
}

/*
maxDepth stops converting a value nested deeper than any page needs,
such as one that refers to itself.
*/
const maxDepth = 64

func convert(v reflect.Value, depth int) any {
	if !v.IsValid() || depth > maxDepth {
		return nil
	}

	switch x := v.Interface().(type) {
	case HTML:
		return x
	case time.Time:
		return x
	}

	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			return nil
		}
		return convert(v.Elem(), depth+1)
	case reflect.Bool:
		return v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return int(v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return unsigned(v.Uint())
	case reflect.Float32, reflect.Float64:
		return v.Float()
	case reflect.String:
		return v.String()
	case reflect.Slice, reflect.Array:
		return convertList(v, depth)
	case reflect.Map:
		return convertMap(v, depth)
	case reflect.Struct:
		return structMap(v, depth)
	case reflect.Invalid, reflect.Uintptr, reflect.Complex64, reflect.Complex128,
		reflect.Chan, reflect.Func, reflect.UnsafePointer:
	}

	return nil
}

/*
unsigned reads an unsigned number as an int, or as a float when it is
too big for one.
*/
func unsigned(u uint64) any {
	if u > math.MaxInt {
		return float64(u)
	}
	return int(u)
}

/*
convertList converts a slice or an array, a nil slice to an empty
list.
*/
func convertList(v reflect.Value, depth int) any {
	if v.Kind() == reflect.Slice && v.IsNil() {
		return []any{}
	}
	out := make([]any, v.Len())
	for i := range out {
		out[i] = convert(v.Index(i), depth+1)
	}
	return out
}

/*
convertMap converts a map with string keys, and reads any other map as
nil.
*/
func convertMap(v reflect.Value, depth int) any {
	if v.Type().Key().Kind() != reflect.String {
		return nil
	}
	out := make(map[string]any, v.Len())
	iter := v.MapRange()
	for iter.Next() {
		out[iter.Key().String()] = convert(iter.Value(), depth+1)
	}
	return out
}

/*
structMap reads a struct's exported fields into a map. The fields of an
embedded struct join the map, unless a field of the same name is
already there.
*/
func structMap(v reflect.Value, depth int) map[string]any {
	t := v.Type()
	out := make(map[string]any, t.NumField())
	for i := range t.NumField() {
		f := t.Field(i)
		name := f.Tag.Get("liquid")
		if !f.IsExported() || name == "-" {
			continue
		}
		if f.Anonymous && name == "" && f.Type.Kind() == reflect.Struct {
			embed(out, structMap(v.Field(i), depth+1))
			continue
		}
		if name == "" {
			name = SnakeCase(f.Name)
		}
		out[name] = convert(v.Field(i), depth+1)
	}
	return out
}

/*
embed adds the fields of an embedded struct the map does not have yet.
*/
func embed(out, fields map[string]any) {
	for k, val := range fields {
		if _, taken := out[k]; !taken {
			out[k] = val
		}
	}
}

/*
SnakeCase writes a Go name in snake case, such as ServerURL as
server_url and HTTPPort as http_port.
*/
func SnakeCase(name string) string {
	runes := []rune(name)
	var b strings.Builder
	for i, r := range runes {
		if unicode.IsUpper(r) {
			startsWord := i > 0 && (unicode.IsLower(runes[i-1]) ||
				(i+1 < len(runes) && unicode.IsLower(runes[i+1]) && unicode.IsUpper(runes[i-1])))
			if startsWord {
				b.WriteByte('_')
			}
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

/*
truthy says whether a value counts as true: everything but nil and
false does, the empty string and zero included.
*/
func truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	}
	return true
}

/*
emptyValue is the value of the empty keyword, and blankValue that of
blank.
*/
type emptyValue struct{}

type blankValue struct{}

/*
isEmpty says whether a value is the empty string, list or map.
*/
func isEmpty(v any) bool {
	switch x := v.(type) {
	case string:
		return x == ""
	case HTML:
		return x == ""
	case []any:
		return len(x) == 0
	case map[string]any:
		return len(x) == 0
	}
	return false
}

/*
isBlank says whether a value is nil, false, empty or only white space.
*/
func isBlank(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case bool:
		return !x
	case string:
		return strings.TrimSpace(x) == ""
	case HTML:
		return strings.TrimSpace(string(x)) == ""
	}
	return isEmpty(v)
}

/*
number reads a value as a number, and false when it is none. Strings
holding a number count, as Liquid lets "3" | plus: 1 read 4.
*/
func number(v any) (float64, bool, bool) {
	switch x := v.(type) {
	case int:
		return float64(x), true, true
	case float64:
		return x, false, true
	case string:
		s := strings.TrimSpace(x)
		if i, err := strconv.Atoi(s); err == nil {
			return float64(i), true, true
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f, false, true
		}
	}
	return 0, false, false
}

/*
toInt reads a value as a whole number, zero when it is none.
*/
func toInt(v any) int {
	f, _, ok := number(v)
	if !ok || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0
	}
	return int(f)
}

/*
equal says whether two values are the same: numbers by value, text and
HTML by their text, lists and maps by their contents.
*/
func equal(a, b any) bool {
	switch b.(type) {
	case emptyValue:
		return isEmpty(a)
	case blankValue:
		return isBlank(a)
	}
	switch a.(type) {
	case emptyValue:
		return isEmpty(b)
	case blankValue:
		return isBlank(b)
	}
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if fa, _, ok := numeric(a); ok {
		if fb, _, ok := numeric(b); ok {
			return fa == fb
		}
		return false
	}
	if sa, ok := text(a); ok {
		if sb, ok := text(b); ok {
			return sa == sb
		}
		return false
	}
	if ta, ok := a.(time.Time); ok {
		tb, ok := b.(time.Time)
		return ok && ta.Equal(tb)
	}
	return reflect.DeepEqual(a, b)
}

/*
numeric reads only real numbers, not text that holds one, as equality
in Liquid keeps 1 and "1" apart.
*/
func numeric(v any) (float64, bool, bool) {
	switch x := v.(type) {
	case int:
		return float64(x), true, true
	case float64:
		return x, false, true
	}
	return 0, false, false
}

func text(v any) (string, bool) {
	switch x := v.(type) {
	case string:
		return x, true
	case HTML:
		return string(x), true
	}
	return "", false
}

/*
compare orders two numbers, two texts or two times, and false when they
cannot be ordered.
*/
func compare(a, b any) (int, bool) {
	if fa, _, ok := numeric(a); ok {
		if fb, _, ok := numeric(b); ok {
			switch {
			case fa < fb:
				return -1, true
			case fa > fb:
				return 1, true
			}
			return 0, true
		}
		return 0, false
	}
	if sa, ok := text(a); ok {
		if sb, ok := text(b); ok {
			return strings.Compare(sa, sb), true
		}
		return 0, false
	}
	if ta, ok := a.(time.Time); ok {
		if tb, ok := b.(time.Time); ok {
			return ta.Compare(tb), true
		}
	}
	return 0, false
}

/*
contains says whether a text holds another, or a list an item.
*/
func contains(haystack, needle any) bool {
	switch x := haystack.(type) {
	case string, HTML:
		s, _ := text(x)
		return strings.Contains(s, toString(needle))
	case []any:
		for _, item := range x {
			if equal(item, needle) {
				return true
			}
		}
	case map[string]any:
		_, ok := x[toString(needle)]
		return ok
	}
	return false
}

/*
toString prints a value as text: nil as nothing, numbers without a
needless fraction, lists as their items run together and times in
RFC 3339.
*/
func toString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case HTML:
		return string(x)
	case bool:
		return strconv.FormatBool(x)
	case int:
		return strconv.Itoa(x)
	case float64:
		return formatFloat(x)
	case time.Time:
		return x.Format(time.RFC3339)
	case []any:
		var b strings.Builder
		for _, item := range x {
			b.WriteString(toString(item))
		}
		return b.String()
	case map[string]any:
		return mapString(x)
	case emptyValue, blankValue:
		return ""
	}
	return ""
}

/*
mapString prints a map as Ruby's Liquid does, its keys in order.
*/
func mapString(x map[string]any) string {
	keys := make([]string, 0, len(x))
	for k := range x {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%q=>%s", k, toString(x[k]))
	}
	b.WriteByte('}')
	return b.String()
}

func formatFloat(f float64) string {
	if f == math.Trunc(f) && math.Abs(f) < 1e15 {
		return strconv.FormatFloat(f, 'f', 1, 64)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

/*
printed is a value as output writes it: HTML as it is, anything else
escaped.
*/
func printed(v any) string {
	if h, ok := v.(HTML); ok {
		return string(h)
	}
	return html.EscapeString(toString(v))
}

/*
list reads a value as a list to loop over: a list as it is, a map as
its key and value pairs in key order, nil as nothing and anything else
as a list of itself.
*/
func list(v any) []any {
	switch x := v.(type) {
	case nil:
		return nil
	case []any:
		return x
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make([]any, len(keys))
		for i, k := range keys {
			out[i] = []any{k, x[k]}
		}
		return out
	case string:
		if x == "" {
			return nil
		}
	}
	return []any{v}
}

/*
size is how many items a list or map holds, or characters a text.
*/
func size(v any) int {
	switch x := v.(type) {
	case string:
		return len([]rune(x))
	case HTML:
		return len([]rune(string(x)))
	case []any:
		return len(x)
	case map[string]any:
		return len(x)
	}
	return 0
}
