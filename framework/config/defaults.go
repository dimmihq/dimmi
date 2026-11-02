/*
|--------------------------------------------------------------------------
| Struct Defaults
|--------------------------------------------------------------------------
|
| A configuration struct states each default beside its field, in a
| `default` tag, so the struct alone documents what an empty
| configuration means. Only zero fields take their default: a value set
| before loading always stays. Nested structs, and pointers to structs
| that are set or carry a tag, get their own defaults; maps are left
| alone because their keys are not known until configuration is read.
|
| Defaults are applied by walking struct fields only. Methods are never
| looked up by name, so the linker can still drop unused methods from
| the binary.
|
*/

package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

var (
	errNotStructPointer = errors.New("config: target must be a pointer to a struct")
	errUnsupportedKind  = errors.New("config: default tag not supported for this type")
)

/*
applyDefaults sets every zero field of the struct the target points at
to the value of its `default` tag.
*/
func applyDefaults(target any) error {
	v := reflect.ValueOf(target)
	if v.Kind() != reflect.Pointer || v.IsNil() || v.Elem().Kind() != reflect.Struct {
		return errNotStructPointer
	}

	return defaultStruct(v.Elem())
}

func defaultStruct(v reflect.Value) error {
	t := v.Type()

	for i := range t.NumField() {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		if err := defaultField(v.Field(i), field.Tag.Get("default")); err != nil {
			return fmt.Errorf("default %s: %w", field.Name, err)
		}
	}

	return nil
}

/*
defaultField fills one field. A field with an empty or missing tag has
no default, though the structs inside it may.
*/
func defaultField(v reflect.Value, tag string) error {
	if v.Kind() == reflect.Struct {
		return defaultStruct(v)
	}

	if v.Kind() == reflect.Pointer && v.Type().Elem().Kind() == reflect.Struct {
		return defaultPointer(v, tag)
	}

	if tag == "" || tag == "-" || !v.IsZero() {
		return nil
	}

	if v.Kind() == reflect.Slice {
		return setSlice(v, tag)
	}

	return setScalar(v, tag)
}

/*
defaultPointer fills the struct a pointer field points at. A nil pointer
is allocated only when its field carries a tag, so optional sections
stay nil unless the struct asks for them.
*/
func defaultPointer(v reflect.Value, tag string) error {
	if v.IsNil() {
		if tag == "" || tag == "-" {
			return nil
		}

		v.Set(reflect.New(v.Type().Elem()))
	}

	return defaultStruct(v.Elem())
}

/*
setSlice fills a slice from a JSON array, such as ["en","fr"], or from a
comma-separated list, such as en, fr.
*/
func setSlice(v reflect.Value, tag string) error {
	if strings.HasPrefix(strings.TrimSpace(tag), "[") {
		if err := json.Unmarshal([]byte(tag), v.Addr().Interface()); err != nil {
			return fmt.Errorf("parse %q: %w", tag, err)
		}

		return nil
	}

	items := splitList(tag)
	slice := reflect.MakeSlice(v.Type(), len(items), len(items))

	for i, item := range items {
		if err := setScalar(slice.Index(i), item); err != nil {
			return err
		}
	}

	v.Set(slice)

	return nil
}

func setScalar(v reflect.Value, s string) error {
	switch {
	case v.Type() == reflect.TypeFor[time.Duration]():
		d, err := time.ParseDuration(s)
		if err != nil {
			return fmt.Errorf("parse %q: %w", s, err)
		}

		v.SetInt(int64(d))
	case v.Kind() == reflect.String:
		v.SetString(s)
	case v.Kind() == reflect.Bool:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return fmt.Errorf("parse %q: %w", s, err)
		}

		v.SetBool(b)
	default:
		return setNumber(v, s)
	}

	return nil
}

func setNumber(v reflect.Value, s string) error {
	var err error

	switch {
	case v.CanInt():
		var n int64
		n, err = strconv.ParseInt(s, 0, v.Type().Bits())
		v.SetInt(n)
	case v.CanUint():
		var n uint64
		n, err = strconv.ParseUint(s, 0, v.Type().Bits())
		v.SetUint(n)
	case v.CanFloat():
		var n float64
		n, err = strconv.ParseFloat(s, v.Type().Bits())
		v.SetFloat(n)
	default:
		return fmt.Errorf("%w: %s", errUnsupportedKind, v.Type())
	}

	if err != nil {
		return fmt.Errorf("parse %q: %w", s, err)
	}

	return nil
}
