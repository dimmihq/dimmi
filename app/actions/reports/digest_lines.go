/*
|--------------------------------------------------------------------------
| Digest Lines
|--------------------------------------------------------------------------
|
| A digest shows each headline number as text: counts with separators,
| percents whole, durations in the two largest units, money in dollars,
| and how it moved since the period before, in percent, or in points for
| a percent.
|
*/

package reports

import (
	"fmt"
	"math"
	"strconv"

	appmail "github.com/dimmihq/dimmi/app/mail"
)

/*
DigestLines writes the numbers as a digest shows them.
*/
func DigestLines(metrics []Metric) []appmail.DigestLine {
	out := make([]appmail.DigestLine, 0, len(metrics))
	for _, m := range metrics {
		out = append(out, appmail.DigestLine{
			Label: m.Label, Value: FormatValue(m.Unit, m.Value), Change: change(m),
		})
	}

	return out
}

/*
FormatValue writes a number in its unit, a dash for none.
*/
func FormatValue(unit string, v *float64) string {
	if v == nil {
		return "–"
	}

	switch unit {
	case UnitPercent:
		return strconv.Itoa(int(math.Round(*v))) + "%"
	case UnitSeconds:
		return FormatSeconds(*v)
	case UnitMicros:
		return fmt.Sprintf("$%.2f", *v/1e6)
	case UnitScore:
		return fmt.Sprintf("%.1f", *v)
	}

	return thousands(int64(math.Round(*v)))
}

/*
FormatSeconds writes a duration in its two largest units.
*/
func FormatSeconds(s float64) string {
	total := int64(math.Round(s))

	switch {
	case total < 60:
		return fmt.Sprintf("%ds", total)
	case total < 3600:
		return fmt.Sprintf("%dm %ds", total/60, total%60)
	case total < 86400:
		return fmt.Sprintf("%dh %dm", total/3600, total%3600/60)
	}

	return fmt.Sprintf("%dd %dh", total/86400, total%86400/3600)
}

/*
thousands writes a whole number with comma separators.
*/
func thousands(n int64) string {
	s := strconv.FormatInt(n, 10)
	if n < 0 {
		return "-" + thousands(-n)
	}

	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}

	return s
}

/*
change writes how a number moved since the period before, empty
without both.
*/
func change(m Metric) string {
	if m.Value == nil || m.Previous == nil {
		return ""
	}

	if m.Unit == UnitPercent {
		return fmt.Sprintf("%+.0f pts", *m.Value-*m.Previous)
	}

	if *m.Previous == 0 {
		return ""
	}

	return fmt.Sprintf("%+.0f%%", (*m.Value-*m.Previous)*100 / *m.Previous)
}
