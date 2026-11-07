/*
|--------------------------------------------------------------------------
| Condition Values
|--------------------------------------------------------------------------
|
| Each operator expects one shape of value: nothing, a text, a list of
| texts, a number, a range, a count of days or a date. Values are parsed
| and folded once at compile time, so evaluation only compares.
|
*/

package audience

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/support/str"
)

/*
maxDays bounds relative date conditions to ten years.
*/
const maxDays = 3650

/*
operand is a parsed condition value. Texts are folded for comparison.
Scalar holds the value of an equality on a field of unknown type.
*/
type operand struct {
	texts   []string
	num     float64
	lo, hi  float64
	days    int
	date    time.Time
	day     string
	dayOnly bool
	scalar  any
}

/*
Value parsing failures.
*/
var (
	errNoValue    = errors.New("this operator takes no value")
	errText       = errors.New("enter a text value")
	errTexts      = errors.New("enter between 1 and 50 values")
	errNumber     = errors.New("enter a number")
	errRange      = errors.New("enter two numbers, the smaller first")
	errDays       = errors.New("enter a whole number of days between 0 and 3650")
	errDate       = errors.New("enter a date as YYYY-MM-DD")
	errScalar     = errors.New("enter a text, a number or true or false")
	errWithinDays = errors.New("within_days applies to event counts only, from 1 to 90")
)

/*
parseOperand reads the leaf's value in the shape its operator expects.
*/
func parseOperand(f Field, r Rule) (operand, error) {
	if r.WithinDays != 0 && (!strings.HasPrefix(r.Field, "event.") || r.WithinDays < 1 || r.WithinDays > 90) {
		return operand{}, errWithinDays
	}

	shape := valueShape(r.Op)
	if r.Op == OpEq || r.Op == OpNeq {
		shape = equalityShape(f.Type)
	}

	if f.Type == TypeSet && r.Op == OpHas {
		shape = valueText
	}

	o, err := parseShape(shape, r.Value)
	if err != nil || (f.Type != TypeID && f.Type != TypeSaved) {
		return o, err
	}

	o.texts = exactTexts(r.Value)

	return o, nil
}

/*
exactTexts reads the value's texts without folding, for ids, which are
compared exactly.
*/
func exactTexts(raw json.RawMessage) []string {
	var one string
	if json.Unmarshal(raw, &one) == nil {
		return []string{one}
	}

	var many []string
	_ = json.Unmarshal(raw, &many)

	return many
}

/*
equalityShape says what an equality compares for a field type.
*/
func equalityShape(t FieldType) string {
	switch t {
	case TypeNumber:
		return valueNumber
	case TypeAuto:
		return "scalar"
	case TypeString, TypeBool, TypeDate, TypeList, TypeSet, TypeURL, TypeID, TypeSaved:
		return valueText
	}

	return valueText
}

/*
parseShape decodes a raw value into the shape.
*/
func parseShape(shape string, raw json.RawMessage) (operand, error) {
	absent := len(raw) == 0 || string(raw) == "null"
	if shape == valueNone {
		if !absent {
			return operand{}, errNoValue
		}

		return operand{}, nil
	}

	switch shape {
	case valueTexts:
		return parseTexts(raw)
	case valueNumber:
		return parseNumber(raw)
	case valueRange:
		return parseRange(raw)
	case valueDays:
		return parseDays(raw)
	case valueDate:
		return parseDate(raw)
	case "scalar":
		return parseScalar(raw)
	}

	var s string
	if json.Unmarshal(raw, &s) != nil || strings.TrimSpace(s) == "" {
		return operand{}, errText
	}

	return operand{texts: []string{str.Fold(s)}}, nil
}

/*
parseTexts reads a list of texts; numbers are accepted as their text.
*/
func parseTexts(raw json.RawMessage) (operand, error) {
	var items []any
	if json.Unmarshal(raw, &items) != nil || len(items) == 0 || len(items) > MaxInValues {
		return operand{}, errTexts
	}

	out := make([]string, 0, len(items))
	for _, item := range items {
		switch v := item.(type) {
		case string:
			if strings.TrimSpace(v) == "" {
				return operand{}, errTexts
			}

			out = append(out, str.Fold(v))
		case float64:
			out = append(out, strconv.FormatFloat(v, 'f', -1, 64))
		default:
			return operand{}, errTexts
		}
	}

	return operand{texts: out}, nil
}

/*
parseNumber reads one number.
*/
func parseNumber(raw json.RawMessage) (operand, error) {
	var n float64
	if json.Unmarshal(raw, &n) != nil {
		return operand{}, errNumber
	}

	return operand{num: n}, nil
}

/*
parseRange reads two numbers, lowest first.
*/
func parseRange(raw json.RawMessage) (operand, error) {
	var pair []float64
	if json.Unmarshal(raw, &pair) != nil || len(pair) != 2 || pair[0] > pair[1] {
		return operand{}, errRange
	}

	return operand{lo: pair[0], hi: pair[1]}, nil
}

/*
parseDays reads a whole count of days.
*/
func parseDays(raw json.RawMessage) (operand, error) {
	var n float64
	if json.Unmarshal(raw, &n) != nil || n != float64(int(n)) || n < 0 || n > maxDays {
		return operand{}, errDays
	}

	return operand{days: int(n)}, nil
}

/*
parseDate reads a calendar day or a full timestamp.
*/
func parseDate(raw json.RawMessage) (operand, error) {
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return operand{}, errDate
	}

	if d, err := time.Parse(time.DateOnly, s); err == nil {
		return operand{date: d, day: s, dayOnly: true}, nil
	}

	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return operand{}, errDate
	}

	return operand{date: t}, nil
}

/*
parseScalar reads a text, a number or a boolean.
*/
func parseScalar(raw json.RawMessage) (operand, error) {
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return operand{}, errScalar
	}

	switch x := v.(type) {
	case string:
		if strings.TrimSpace(x) == "" {
			return operand{}, errScalar
		}

		return operand{scalar: str.Fold(x), texts: []string{str.Fold(x)}}, nil
	case float64, bool:
		return operand{scalar: x}, nil
	}

	return operand{}, errScalar
}
