/*
|--------------------------------------------------------------------------
| Imported Values
|--------------------------------------------------------------------------
|
| Another tool's values are looser than a ticket's: numbers arrive as
| text, dates as timestamps, an option the field no longer offers. Each
| value is made to fit its field before the create checks it; one that
| cannot fit is left out rather than keeping the whole ticket out.
|
*/

package ticketimports

import (
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/app/models"
)

/*
fit returns the values that fit the type's fields, by key.
*/
func fit(fields []models.TicketField, values map[string]any) map[string]any {
	out := map[string]any{}

	for _, f := range fields {
		v, ok := values[f.Key]
		if !ok || v == nil || f.ArchivedAt != nil {
			continue
		}

		if fitted, ok := fitOne(f, v); ok {
			out[f.Key] = fitted
		}
	}

	return out
}

/*
fitOne makes one value the field's type.
*/
func fitOne(f models.TicketField, v any) (any, bool) {
	switch f.DataType {
	case "short_text":
		return cut(text(v), 255), text(v) != ""
	case "long_text":
		return cut(text(v), 10000), text(v) != ""
	case "number", "decimal", "currency":
		return number(f.DataType, v)
	case "boolean":
		b, ok := v.(bool)

		return b, ok
	case "date":
		return date(v)
	case "single_choice":
		s := text(v)

		return s, slices.Contains(f.Options, s)
	case "multiple_choice":
		return choices(f.Options, v)
	}

	return nil, false
}

/*
number reads a number or numeric text; a number field keeps whole
numbers only.
*/
func number(dataType string, v any) (any, bool) {
	n, err := strconv.ParseFloat(strings.TrimSpace(text(v)), 64)
	if err != nil {
		return nil, false
	}

	if dataType == "number" && n != math.Trunc(n) {
		return nil, false
	}

	return n, true
}

/*
date reads a date, a timestamp or seconds since the epoch as a day.
*/
func date(v any) (any, bool) {
	if n, ok := v.(float64); ok {
		return unix(int64(n)).Format(time.DateOnly), true
	}

	s := strings.TrimSpace(text(v))
	for _, layout := range []string{time.DateOnly, time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format(time.DateOnly), true
		}
	}

	return nil, false
}

/*
choices keeps the options the field offers.
*/
func choices(options []string, v any) (any, bool) {
	list, ok := v.([]any)
	if !ok {
		list = []any{v}
	}

	out := []any{}

	for _, x := range list {
		if s := text(x); slices.Contains(options, s) && !slices.Contains(out, any(s)) {
			out = append(out, s)
		}
	}

	return out, len(out) > 0
}
