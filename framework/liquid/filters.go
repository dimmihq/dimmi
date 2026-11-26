/*
|--------------------------------------------------------------------------
| Standard Filters
|--------------------------------------------------------------------------
|
| Liquid's own filters, for text, numbers, lists and dates, behaving as
| Shopify's do. Filters that would grow text or lists without bound,
| such as a join of a huge list, are held by the render's output limit
| once printed, and by the size of their input before that.
|
*/

package liquid

import (
	"encoding/json"
	"fmt"
	"html"
	"maps"
	"math"
	"net/url"
	"regexp"
	"strings"
	"unicode"
)

/*
maxText is the longest text a filter builds, so a template cannot fill
memory with a text it never prints.
*/
const maxText = 4 << 20

/*
standardFilters returns Liquid's own filters by name.
*/
func standardFilters() map[string]Filter {
	filters := map[string]Filter{}
	maps.Copy(filters, numberFilters())
	maps.Copy(filters, textFilters())
	maps.Copy(filters, listFilters())
	return filters
}

/*
numberFilters are the filters for numbers.
*/
func numberFilters() map[string]Filter {
	return map[string]Filter{
		"abs":        numberFilter(math.Abs),
		"ceil":       numberFilter(math.Ceil),
		"floor":      numberFilter(math.Floor),
		"round":      roundFilter,
		"plus":       arithmetic(func(a, b float64) float64 { return a + b }),
		"minus":      arithmetic(func(a, b float64) float64 { return a - b }),
		"times":      arithmetic(func(a, b float64) float64 { return a * b }),
		"divided_by": dividedBy,
		"modulo":     modulo,
		"at_least":   arithmetic(math.Max),
		"at_most":    arithmetic(math.Min),
	}
}

/*
textFilters are the filters for text, escaping and dates.
*/
func textFilters() map[string]Filter {
	return map[string]Filter{
		"append":         textFilter(func(s string, c *Call) string { return s + toString(c.Arg(0)) }),
		"prepend":        textFilter(func(s string, c *Call) string { return toString(c.Arg(0)) + s }),
		"capitalize":     textFilter(capitalize),
		"downcase":       textFilter(func(s string, _ *Call) string { return strings.ToLower(s) }),
		"upcase":         textFilter(func(s string, _ *Call) string { return strings.ToUpper(s) }),
		"strip":          textFilter(func(s string, _ *Call) string { return strings.TrimSpace(s) }),
		"lstrip":         textFilter(lstrip),
		"rstrip":         textFilter(rstrip),
		"strip_newlines": textFilter(stripNewlines),
		"strip_html":     textFilter(stripHTML),
		"remove":         textFilter(remove),
		"remove_first":   textFilter(removeFirst),
		"replace":        textFilter(replace),
		"replace_first":  textFilter(replaceFirst),
		"truncate":       textFilter(truncate),
		"truncatewords":  textFilter(truncateWords),
		"url_encode":     textFilter(func(s string, _ *Call) string { return url.QueryEscape(s) }),
		"url_decode":     textFilter(urlDecode),
		"escape":         escapeFilter,
		"escape_once":    escapeOnce,
		"newline_to_br":  newlineToBr,
		"raw":            func(c *Call) (any, error) { return HTML(toString(c.Input)), nil },
		"default":        defaultFilter,
		"date":           dateFilter,
		"json":           jsonFilter,
	}
}

func checkText(s string) (string, error) {
	if len(s) > maxText {
		return "", fmt.Errorf("%w: text longer than %d bytes", ErrLimit, maxText)
	}
	return s, nil
}

func textFilter(f func(string, *Call) string) Filter {
	return func(c *Call) (any, error) {
		return checkText(f(toString(c.Input), c))
	}
}

func numberFilter(f func(float64) float64) Filter {
	return func(c *Call) (any, error) {
		n, whole, _ := number(c.Input)
		return numeral{f(n), whole}.result(), nil
	}
}

/*
numeral is a filter's numeric result, whole when every number it came
from was.
*/
type numeral struct {
	value float64
	whole bool
}

/*
result is the numeral as a template reads it: an int when it is whole
and fits, else a float.
*/
func (n numeral) result() any {
	if n.whole && n.value == math.Trunc(n.value) && math.Abs(n.value) < 1<<53 {
		return int(n.value)
	}
	return n.value
}

func arithmetic(f func(a, b float64) float64) Filter {
	return func(c *Call) (any, error) {
		a, aWhole, _ := number(c.Input)
		b, bWhole, _ := number(c.Arg(0))
		return numeral{f(a, b), aWhole && bWhole}.result(), nil
	}
}

func roundFilter(c *Call) (any, error) {
	n, _, _ := number(c.Input)
	places := toInt(c.Arg(0))
	if places <= 0 {
		return int(math.Round(n)), nil
	}
	scale := math.Pow(10, float64(min(places, 12)))
	return math.Round(n*scale) / scale, nil
}

func dividedBy(c *Call) (any, error) {
	a, aWhole, _ := number(c.Input)
	b, bWhole, _ := number(c.Arg(0))
	if b == 0 {
		return nil, fmt.Errorf("divided by 0")
	}
	if aWhole && bWhole {
		return int(math.Floor(a / b)), nil
	}
	return a / b, nil
}

func modulo(c *Call) (any, error) {
	a, aWhole, _ := number(c.Input)
	b, bWhole, _ := number(c.Arg(0))
	if b == 0 {
		return nil, fmt.Errorf("divided by 0")
	}
	return numeral{math.Mod(a, b), aWhole && bWhole}.result(), nil
}

func capitalize(s string, _ *Call) string {
	for i, r := range s {
		return string(unicode.ToUpper(r)) + s[i+len(string(r)):]
	}
	return s
}

var tags = regexp.MustCompile(`(?s)<script.*?</script>|<style.*?</style>|<!--.*?-->|<[^>]*>`)

func lstrip(s string, _ *Call) string {
	return strings.TrimLeftFunc(s, unicode.IsSpace)
}

func rstrip(s string, _ *Call) string {
	return strings.TrimRightFunc(s, unicode.IsSpace)
}

func stripNewlines(s string, _ *Call) string {
	return strings.NewReplacer("\r", "", "\n", "").Replace(s)
}

func remove(s string, c *Call) string {
	return strings.ReplaceAll(s, toString(c.Arg(0)), "")
}

func removeFirst(s string, c *Call) string {
	return strings.Replace(s, toString(c.Arg(0)), "", 1)
}

func replace(s string, c *Call) string {
	return strings.ReplaceAll(s, toString(c.Arg(0)), toString(c.Arg(1)))
}

func replaceFirst(s string, c *Call) string {
	return strings.Replace(s, toString(c.Arg(0)), toString(c.Arg(1)), 1)
}

func stripHTML(s string, _ *Call) string {
	return html.UnescapeString(tags.ReplaceAllString(s, ""))
}

func truncate(s string, c *Call) string {
	length := 50
	if c.Arg(0) != nil {
		length = toInt(c.Arg(0))
	}
	ellipsis := "..."
	if c.Arg(1) != nil {
		ellipsis = toString(c.Arg(1))
	}
	runes := []rune(s)
	if len(runes) <= length {
		return s
	}
	keep := max(0, length-len([]rune(ellipsis)))
	return string(runes[:keep]) + ellipsis
}

func truncateWords(s string, c *Call) string {
	count := 15
	if c.Arg(0) != nil {
		count = max(1, toInt(c.Arg(0)))
	}
	ellipsis := "..."
	if c.Arg(1) != nil {
		ellipsis = toString(c.Arg(1))
	}
	words := strings.Fields(s)
	if len(words) <= count {
		return s
	}
	return strings.Join(words[:count], " ") + ellipsis
}

func urlDecode(s string, _ *Call) string {
	out, err := url.QueryUnescape(s)
	if err != nil {
		return s
	}
	return out
}

func escapeFilter(c *Call) (any, error) {
	if c.Input == nil {
		return nil, nil
	}
	return HTML(html.EscapeString(toString(c.Input))), nil
}

func escapeOnce(c *Call) (any, error) {
	return HTML(html.EscapeString(html.UnescapeString(toString(c.Input)))), nil
}

func newlineToBr(c *Call) (any, error) {
	s := printed(c.Input)
	return HTML(strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\n", "<br />\n")), nil
}

func defaultFilter(c *Call) (any, error) {
	allowFalse := truthy(c.Named["allow_false"])
	in := c.Input
	if allowFalse && in == false {
		return in, nil
	}
	if !truthy(in) || isEmpty(in) {
		return c.Arg(0), nil
	}
	return in, nil
}

func jsonFilter(c *Call) (any, error) {
	out, err := json.Marshal(jsonable(c.Input))
	if err != nil {
		return nil, err
	}
	return checkText(string(out))
}

func jsonable(v any) any {
	switch x := v.(type) {
	case HTML:
		return string(x)
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = jsonable(item)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, item := range x {
			out[k] = jsonable(item)
		}
		return out
	case emptyValue, blankValue:
		return nil
	}
	return v
}
