/*
|--------------------------------------------------------------------------
| Built-in Rules
|--------------------------------------------------------------------------
|
| The rules every application needs, named as the validate tag writes
| them. Size rules measure what a person would count: the characters of
| a string, the items of a list or map, the value of a number. A time is
| compared with the present, so `validate:"gt"` on a time means "in the
| future".
|
| A rule given a value it cannot judge, such as email on a number, or a
| parameter it cannot read, such as max=ten, is a mistake in the code
| rather than in the input, and fails the validation with ErrInvalidRule
| instead of blaming the client.
|
*/

package validation

import (
	"fmt"
	"net/mail"
	"net/url"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

/*
check reports whether the value passes a rule with the parameter. The
value is never a pointer or an interface. The error means the rule does
not apply to the value or cannot read the parameter.
*/
type check func(f *Factory, v reflect.Value, param string) (bool, error)

/*
builtins returns the rules the factory provides by name.
*/
func builtins() map[string]check {
	return map[string]check{
		"required": func(_ *Factory, v reflect.Value, _ string) (bool, error) { return present(v), nil },
		"len":      sized(func(c int) bool { return c == 0 }),
		"min":      sized(func(c int) bool { return c >= 0 }),
		"max":      sized(func(c int) bool { return c <= 0 }),
		"gt":       sized(func(c int) bool { return c > 0 }),
		"gte":      sized(func(c int) bool { return c >= 0 }),
		"lt":       sized(func(c int) bool { return c < 0 }),
		"lte":      sized(func(c int) bool { return c <= 0 }),
		"eq":       equal,
		"ne":       notEqual,
		"oneof":    oneOf,
		"email":    text(isEmail),
		"url":      text(isURL),
		"uuid":     text(isUUID),
		"alpha":    text(func(s string) bool { return s != "" && strings.Trim(s, letters()) == "" }),
		"alphanum": text(func(s string) bool { return s != "" && strings.Trim(s, letters()+digits()) == "" }),
	}
}

/*
present reports whether a required value was given: a non-empty string
or collection, a non-zero number, true, or a non-zero struct.
*/
func present(v reflect.Value) bool {
	if isCollection(v.Kind()) {
		return v.Len() > 0
	}

	return !v.IsZero()
}

/*
sized turns a test of a comparison's sign into a size rule.
*/
func sized(passes func(c int) bool) check {
	return func(f *Factory, v reflect.Value, param string) (bool, error) {
		c, err := f.compare(v, param)
		if err != nil {
			return false, err
		}

		return passes(c), nil
	}
}

/*
compare measures the value against the parameter and returns -1, 0 or
1 as the measure is below, at or above it. A time with no parameter is
compared with the present.
*/
func (f *Factory) compare(v reflect.Value, param string) (int, error) {
	if v.Type() == reflect.TypeFor[time.Time]() && param == "" {
		at, _ := reflect.TypeAssert[time.Time](v)

		return at.Compare(f.clock.Now()), nil
	}

	k := v.Kind()

	if k == reflect.String {
		return compareInt(int64(utf8.RuneCountInString(v.String())), param)
	}

	if isCollection(k) {
		return compareInt(int64(v.Len()), param)
	}

	if k >= reflect.Int && k <= reflect.Int64 {
		return compareInt(v.Int(), param)
	}

	if k >= reflect.Uint && k <= reflect.Uintptr {
		limit, err := strconv.ParseUint(param, 10, 64)

		return compareOrdered(v.Uint(), limit, param, err)
	}

	if k == reflect.Float32 || k == reflect.Float64 {
		limit, err := strconv.ParseFloat(param, 64)

		return compareOrdered(v.Float(), limit, param, err)
	}

	return 0, fmt.Errorf("%w: cannot measure a %s", ErrInvalidRule, v.Type())
}

/*
isCollection reports whether the kind holds items: a list or a map.
*/
func isCollection(k reflect.Kind) bool {
	return k == reflect.Slice || k == reflect.Array || k == reflect.Map
}

/*
compareInt compares an integer with a parameter that must be one.
*/
func compareInt(measure int64, param string) (int, error) {
	limit, err := strconv.ParseInt(param, 10, 64)

	return compareOrdered(measure, limit, param, err)
}

/*
compareOrdered compares the measure with the parsed parameter, or fails
when the parameter could not be parsed.
*/
func compareOrdered[T int64 | uint64 | float64](measure, limit T, param string, err error) (int, error) {
	if err != nil {
		return 0, fmt.Errorf("%w: parameter %q is not a number", ErrInvalidRule, param)
	}

	switch {
	case measure < limit:
		return -1, nil
	case measure > limit:
		return 1, nil
	default:
		return 0, nil
	}
}

/*
equal compares a string or a boolean with the parameter as written, and
anything else by its size.
*/
func equal(f *Factory, v reflect.Value, param string) (bool, error) {
	if v.Kind() == reflect.String {
		return v.String() == param, nil
	}

	if v.Kind() != reflect.Bool {
		c, err := f.compare(v, param)

		return c == 0, err
	}

	want, err := strconv.ParseBool(param)
	if err != nil {
		return false, fmt.Errorf("%w: parameter %q is not a boolean", ErrInvalidRule, param)
	}

	return v.Bool() == want, nil
}

/*
notEqual is the opposite of equal.
*/
func notEqual(f *Factory, v reflect.Value, param string) (bool, error) {
	same, err := equal(f, v, param)

	return !same, err
}

/*
oneOf reports whether the value equals one of the space-separated
values of the parameter.
*/
func oneOf(f *Factory, v reflect.Value, param string) (bool, error) {
	for allowed := range strings.FieldsSeq(param) {
		same, err := equal(f, v, allowed)
		if err != nil || same {
			return same, err
		}
	}

	return false, nil
}

/*
text turns a test of a string into a rule for string values.
*/
func text(passes func(s string) bool) check {
	return func(_ *Factory, v reflect.Value, _ string) (bool, error) {
		if v.Kind() != reflect.String {
			return false, fmt.Errorf("%w: a %s is not text", ErrInvalidRule, v.Type())
		}

		return passes(v.String()), nil
	}
}

/*
isEmail reports whether the text is a bare email address, without a
display name or surrounding space.
*/
func isEmail(s string) bool {
	address, err := mail.ParseAddress(s)

	return err == nil && address.Name == "" && address.Address == s
}

/*
isURL reports whether the text is an absolute URL with a scheme and a
host, or an opaque one such as mailto:ana@example.com.
*/
func isURL(s string) bool {
	u, err := url.Parse(s)

	return err == nil && u.Scheme != "" && (u.Host != "" || u.Opaque != "")
}

/*
isUUID reports whether the text is a UUID in its canonical form of five
hyphenated groups of hexadecimal digits, in either case.
*/
func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}

	for i, r := range s {
		hyphen := slices.Contains([]int{8, 13, 18, 23}, i)
		if hyphen != (r == '-') || !hyphen && !strings.ContainsRune(digits()+"abcdefABCDEF", r) {
			return false
		}
	}

	return true
}

/*
letters returns the ASCII letters.
*/
func letters() string {
	return "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
}

/*
digits returns the decimal digits.
*/
func digits() string {
	return "0123456789"
}
