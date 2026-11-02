/*
|--------------------------------------------------------------------------
| Walking A Struct
|--------------------------------------------------------------------------
|
| Every exported field is checked against the rules of its tag, in field
| order, and nested structs are walked in place, so "address.city" is
| checked where it sits. Embedded structs add their fields to the struct
| around them, as JSON does. A field stops at its first broken rule, so
| a missing value is reported as missing and not also as too short.
|
| A nil pointer is a value that was not given: only "required" can fail
| on it, and a pointer that is set passes "required" whatever it points
| to, so false or 0 can be sent on purpose. "omitempty" skips the remaining rules for a value not given, or
| for a zero value that is not behind a pointer; a pointer to a zero
| value was given on purpose, such as an empty name sent to clear one,
| and is checked. "dive" applies the rules after it to every item of a
| list or map instead of the list itself, and "keys" up to "endkeys"
| right after it apply to the keys of a map.
|
| Tags are read once per type; the rules they name are resolved when a
| value is checked, so a rule registered at boot is found by every type.
|
*/

package validation

import (
	"context"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"
)

/*
The words of a tag that steer the walk rather than check a value.
*/
const (
	tagOmitEmpty = "omitempty"
	tagDive      = "dive"
	tagKeys      = "keys"
	tagEndKeys   = "endkeys"
)

/*
step is one rule of a tag with its parameter.
*/
type step struct {
	rule  string
	param string
}

/*
chain is what a tag asks of one value: whether an empty value skips the
rules, the rules in order, and for collections the rules of every key
and every item.
*/
type chain struct {
	omitEmpty bool
	steps     []step
	keys      []step
	items     *chain
}

/*
field is one checked field of a struct type: its position, the name
clients know it by, its rules, and whether it is embedded and so adds
its own fields in place.
*/
type field struct {
	index    int
	name     string
	rules    *chain
	embedded bool
}

/*
parseTag reads a validate tag such as "required,dive,email".
*/
func parseTag(tag string) (*chain, error) {
	c := &chain{}
	if tag == "" {
		return c, nil
	}

	parts := strings.Split(tag, ",")

	for i, part := range parts {
		name, param, _ := strings.Cut(part, "=")

		switch name {
		case tagOmitEmpty:
			c.omitEmpty = true
		case tagDive:
			err := c.dive(parts[i+1:])

			return c, err
		case "", tagKeys, tagEndKeys:
			return nil, fmt.Errorf("%w: %q out of place in %q", ErrInvalidRule, name, tag)
		default:
			c.steps = append(c.steps, step{rule: name, param: param})
		}
	}

	return c, nil
}

/*
dive reads what follows "dive": the keys section, if any, then the rules
of every item.
*/
func (c *chain) dive(rest []string) error {
	if len(rest) > 0 && rest[0] == tagKeys {
		end := slices.Index(rest, tagEndKeys)
		if end < 0 {
			return fmt.Errorf("%w: %q without %q", ErrInvalidRule, tagKeys, tagEndKeys)
		}

		keys, err := parseTag(strings.Join(rest[1:end], ","))
		if err != nil {
			return err
		}

		if keys.items != nil || keys.omitEmpty {
			return fmt.Errorf("%w: only rules may check keys", ErrInvalidRule)
		}

		c.keys, rest = keys.steps, rest[end+1:]
	}

	items, err := parseTag(strings.Join(rest, ","))
	c.items = items

	return err
}

/*
fields returns the checked fields of a struct type, read once and then
cached.
*/
func (f *Factory) fields(t reflect.Type) ([]field, error) {
	if cached, ok := f.types.Load(t); ok {
		fields, _ := cached.([]field)

		return fields, nil
	}

	var fields []field

	for i := range t.NumField() {
		sf := t.Field(i)
		tag := sf.Tag.Get("validate")

		if tag == "-" || !sf.IsExported() && !sf.Anonymous {
			continue
		}

		rules, err := parseTag(tag)
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", sf.Name, err)
		}

		name := jsonName(sf)
		fields = append(fields, field{index: i, name: name, rules: rules, embedded: sf.Anonymous && name == sf.Name})
	}

	f.types.Store(t, fields)

	return fields, nil
}

/*
walker checks one value, collecting the broken rules.
*/
type walker struct {
	factory *Factory
	broken  []broken
}

/*
broken is a broken rule waiting for its message: the violation and the
kind of value it broke on, which picks the wording of size rules.
*/
type broken struct {
	violation Violation
	kind      reflect.Kind
}

/*
walkStruct checks every field of a struct value under the path.
*/
func (w *walker) walkStruct(ctx context.Context, v reflect.Value, path string) error {
	fields, err := w.factory.fields(v.Type())
	if err != nil {
		return err
	}

	for _, fd := range fields {
		value := v.Field(fd.index)

		if fd.embedded {
			if err := w.walkEmbedded(ctx, value, path); err != nil {
				return err
			}

			continue
		}

		if err := w.walkValue(ctx, value, join(path, fd.name), fd.rules); err != nil {
			return err
		}
	}

	return nil
}

/*
walkEmbedded checks the fields of an embedded struct as if they were
the outer struct's own.
*/
func (w *walker) walkEmbedded(ctx context.Context, v reflect.Value, path string) error {
	v, given := follow(v)
	if !given || v.Kind() != reflect.Struct {
		return nil
	}

	return w.walkStruct(ctx, v, path)
}

/*
walkValue checks one value against its rules, then its items or its
fields.
*/
func (w *walker) walkValue(ctx context.Context, v reflect.Value, path string, c *chain) error {
	followed, given := follow(v)
	if !given {
		if !c.omitEmpty && slices.ContainsFunc(c.steps, func(s step) bool { return s.rule == "required" }) {
			w.add(path, step{rule: "required"}, reflect.Invalid)
		}

		return nil
	}

	indirect := v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface
	if c.omitEmpty && !indirect && v.IsZero() {
		return nil
	}

	steps := c.steps
	if indirect {
		steps = slices.DeleteFunc(slices.Clone(steps), func(s step) bool { return s.rule == "required" })
	}

	passed, err := w.apply(ctx, followed, path, steps)
	if err != nil || !passed {
		return err
	}

	if c.items != nil {
		return w.walkItems(ctx, followed, path, c)
	}

	if followed.Kind() == reflect.Struct && followed.Type() != reflect.TypeFor[time.Time]() {
		return w.walkStruct(ctx, followed, path)
	}

	return nil
}

/*
walkItems checks every item of a list, and every key and item of a map,
the map's in key order.
*/
func (w *walker) walkItems(ctx context.Context, v reflect.Value, path string, c *chain) error {
	if v.Kind() == reflect.Map {
		return w.walkMap(ctx, v, path, c)
	}

	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		return fmt.Errorf("field %s: %w: cannot dive into a %s", path, ErrInvalidRule, v.Type())
	}

	for i := range v.Len() {
		if err := w.walkValue(ctx, v.Index(i), join(path, strconv.Itoa(i)), c.items); err != nil {
			return err
		}
	}

	return nil
}

/*
walkMap checks the keys and items of a map in the order of their keys.
*/
func (w *walker) walkMap(ctx context.Context, v reflect.Value, path string, c *chain) error {
	keys := map[string]reflect.Value{}
	for _, key := range v.MapKeys() {
		keys[fmt.Sprint(key)] = key
	}

	for _, name := range slices.Sorted(maps.Keys(keys)) {
		key := keys[name]

		passed, err := w.apply(ctx, key, join(path, name), c.keys)
		if err != nil {
			return err
		}

		if !passed {
			continue
		}

		if err := w.walkValue(ctx, v.MapIndex(key), join(path, name), c.items); err != nil {
			return err
		}
	}

	return nil
}

/*
apply runs the rules on a value until one breaks, recording it. It
reports whether every rule passed.
*/
func (w *walker) apply(ctx context.Context, v reflect.Value, path string, steps []step) (bool, error) {
	for _, s := range steps {
		passed, err := w.factory.run(ctx, v, s)
		if err != nil {
			return false, fmt.Errorf("field %s: rule %q: %w", path, s.rule, err)
		}

		if !passed {
			w.add(path, s, v.Kind())

			return false, nil
		}
	}

	return true, nil
}

/*
add records a broken rule.
*/
func (w *walker) add(path string, s step, kind reflect.Kind) {
	w.broken = append(w.broken, broken{violation: Violation{Field: path, Rule: s.rule, Param: s.param}, kind: kind})
}

/*
follow unwraps pointers and interfaces, reporting false for a nil one:
a value that was not given.
*/
func follow(v reflect.Value) (reflect.Value, bool) {
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return v, false
		}

		v = v.Elem()
	}

	return v, true
}

/*
join adds a segment to a dotted field path.
*/
func join(path, segment string) string {
	if path == "" {
		return segment
	}

	return path + "." + segment
}

/*
jsonName names a field by its JSON name, so errors use the names the
client sent. A field without one, or hidden from JSON, keeps its Go
name.
*/
func jsonName(sf reflect.StructField) string {
	name, _, _ := strings.Cut(sf.Tag.Get("json"), ",")
	if name == "" || name == "-" {
		return sf.Name
	}

	return name
}
