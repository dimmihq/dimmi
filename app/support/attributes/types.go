/*
|--------------------------------------------------------------------------
| Attribute Types
|--------------------------------------------------------------------------
|
| A defined attribute holds one type of value, so rules compare like with
| like. A value that does not fit is dropped and reported, never stored.
| Dates arrive as ISO 8601 text or Unix time, in seconds or, as many
| systems send them, milliseconds, and are kept as Unix seconds. A key
| first seen on a trusted write takes the type of its value: a key ending
| in _at, or text that reads as a date, is a date.
|
*/

package attributes

import (
	"encoding/json"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
)

/*
The types an attribute may be defined with.
*/
const (
	TypeString  = "string"
	TypeNumber  = "number"
	TypeBoolean = "boolean"
	TypeDate    = "date"
	TypeList    = "list"
)

/*
The reasons a write is dropped.
*/
const (
	ProblemTypeMismatch    = "type_mismatch"
	ProblemInvalidOption   = "invalid_option"
	ProblemArchivedWrite   = "archived_write"
	ProblemKeyLimitReached = "key_limit_reached"
)

/*
millisecondsAbove is the size past which a number of Unix time is read
as milliseconds: 10^11 seconds is in the year 5138.
*/
const millisecondsAbove = 1e11

/*
Types lists the types, in the order builders offer them.
*/
func Types() []string {
	return []string{TypeString, TypeNumber, TypeBoolean, TypeDate, TypeList}
}

/*
Infer is the type a new key takes from its first value.
*/
func Infer(key string, value any) string {
	switch v := value.(type) {
	case bool:
		return TypeBoolean
	case float64, json.Number:
		if strings.HasSuffix(key, "_at") {
			return TypeDate
		}

		return TypeNumber
	case string:
		if strings.HasSuffix(key, "_at") || isDate(v) {
			return TypeDate
		}
	}

	return TypeString
}

/*
Coerce fits a value to a type: the value to store, or the reason it
does not fit. A null passes, since it removes the key.
*/
func Coerce(dataType string, options []string, value any) (any, string) {
	if value == nil {
		return nil, ""
	}

	switch dataType {
	case TypeNumber:
		return number(value)
	case TypeBoolean:
		if b, ok := value.(bool); ok {
			return b, ""
		}
	case TypeDate:
		return date(value)
	case TypeList:
		s, ok := value.(string)
		if ok && slices.Contains(options, s) {
			return s, ""
		}

		if ok {
			return nil, ProblemInvalidOption
		}
	default:
		if s, ok := value.(string); ok {
			return s, ""
		}
	}

	return nil, ProblemTypeMismatch
}

/*
Shape describes a value without showing it, for problem reports.
*/
func Shape(value any) string {
	switch v := value.(type) {
	case bool:
		return "boolean"
	case float64, json.Number:
		return "number"
	case string:
		return "text of " + strconv.Itoa(len([]rune(v))) + " characters"
	}

	return "unknown"
}

func number(value any) (any, string) {
	switch v := value.(type) {
	case float64:
		return v, ""
	case json.Number:
		f, err := v.Float64()
		if err == nil {
			return f, ""
		}
	}

	return nil, ProblemTypeMismatch
}

func date(value any) (any, string) {
	switch v := value.(type) {
	case float64:
		return unixSeconds(v), ""
	case json.Number:
		f, err := v.Float64()
		if err == nil {
			return unixSeconds(f), ""
		}
	case string:
		if t, ok := parseDate(v); ok {
			return float64(t.Unix()), ""
		}
	}

	return nil, ProblemTypeMismatch
}

/*
unixSeconds reads a number of Unix time, dividing milliseconds.
*/
func unixSeconds(n float64) float64 {
	if math.Abs(n) > millisecondsAbove {
		return math.Floor(n / 1000)
	}

	return math.Floor(n)
}

func parseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{time.RFC3339Nano, time.DateTime, time.DateOnly} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}

	return time.Time{}, false
}

func isDate(s string) bool {
	_, ok := parseDate(s)
	return ok
}
