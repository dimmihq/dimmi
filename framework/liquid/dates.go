/*
|--------------------------------------------------------------------------
| Dates
|--------------------------------------------------------------------------
|
| The date filter formats a time with strftime's directives, as Ruby's
| Liquid does. "now" and "today" read the render's clock, so a test can
| freeze the time a page shows.
|
*/

package liquid

import (
	"fmt"
	"strings"
	"time"
)

/*
dateFilter formats a time with strftime's directives. The input is a
time, "now", "today", a Unix time or a date in RFC 3339 or as a day.
*/
func dateFilter(c *Call) (any, error) {
	t, ok := toTime(c.Input, c.Now)
	if !ok {
		return c.Input, nil
	}
	format := toString(c.Arg(0))
	if format == "" {
		return c.Input, nil
	}
	return Strftime(t, format), nil
}

/*
toTime reads a value as a time, with now for "now" and "today".
*/
func toTime(v any, now time.Time) (time.Time, bool) {
	switch x := v.(type) {
	case time.Time:
		return x, !x.IsZero()
	case int:
		return time.Unix(int64(x), 0).UTC(), true
	case string:
		switch x {
		case "now", "today":
			return now.UTC(), true
		}
		for _, layout := range []string{time.RFC3339, time.DateTime, time.DateOnly} {
			if t, err := time.Parse(layout, x); err == nil {
				return t, true
			}
		}
	}
	return time.Time{}, false
}

/*
Strftime formats a time with strftime's directives, %-d and the like
dropping the padding.
*/
func Strftime(t time.Time, format string) string {
	var b strings.Builder
	for i := 0; i < len(format); i++ {
		if format[i] != '%' || i+1 == len(format) {
			b.WriteByte(format[i])
			continue
		}
		i++
		pad := true
		if format[i] == '-' && i+1 < len(format) {
			pad = false
			i++
		}
		n, width, ok := numberDirective(t, format[i])
		switch {
		case ok && pad:
			fmt.Fprintf(&b, "%0*d", width, n)
		case ok:
			fmt.Fprint(&b, n)
		default:
			b.WriteString(textDirective(t, format[i]))
		}
	}
	return b.String()
}

/*
numberDirective reads the number a directive prints and the width it
pads to, and false for a directive that prints no padded number.
*/
func numberDirective(t time.Time, c byte) (int, int, bool) {
	switch c {
	case 'd':
		return t.Day(), 2, true
	case 'H':
		return t.Hour(), 2, true
	case 'I':
		h := t.Hour() % 12
		if h == 0 {
			h = 12
		}
		return h, 2, true
	case 'j':
		return t.YearDay(), 3, true
	case 'm':
		return int(t.Month()), 2, true
	case 'M':
		return t.Minute(), 2, true
	case 'S':
		return t.Second(), 2, true
	case 'y':
		return t.Year() % 100, 2, true
	}
	return 0, 0, false
}

/*
textDirective prints any other directive. One it does not know prints
as it is written.
*/
func textDirective(t time.Time, c byte) string {
	switch c {
	case 'a':
		return t.Format("Mon")
	case 'A':
		return t.Format("Monday")
	case 'b', 'h':
		return t.Format("Jan")
	case 'B':
		return t.Format("January")
	case 'e':
		return fmt.Sprintf("%2d", t.Day())
	case 'p':
		return t.Format("PM")
	case 'Y':
		return fmt.Sprint(t.Year())
	case 'Z':
		return t.Format("MST")
	case 'z':
		return t.Format("-0700")
	case 'F':
		return t.Format(time.DateOnly)
	case 's':
		return fmt.Sprint(t.Unix())
	case '%':
		return "%"
	}
	return "%" + string(c)
}
