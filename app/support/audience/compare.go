/*
|--------------------------------------------------------------------------
| Comparing Values
|--------------------------------------------------------------------------
|
| The same semantics everywhere: text compares case-insensitively after
| composing letters the canonical way, keyword lists match whole words
| and ignore accents, relative dates count rolling 24-hour days from
| now, and a value of the wrong type never matches rather than failing.
|
*/

package audience

import (
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/dimmihq/dimmi/framework/support/str"

	"github.com/dimmihq/dimmi/app/support/urlmatch"
)

/*
day is one rolling day.
*/
const day = 24 * time.Hour

/*
normalize turns a fact into the form comparisons take: folded text,
float64 numbers, times for date conditions, folded sets.
*/
func normalize(n *node, value any) any {
	switch v := value.(type) {
	case string:
		if dateOp(n.op) {
			return parseTime(v)
		}

		return v
	case int:
		return numberOrTime(n, float64(v))
	case int32:
		return numberOrTime(n, float64(v))
	case int64:
		return numberOrTime(n, float64(v))
	case float64:
		return numberOrTime(n, v)
	case time.Time:
		return v
	case *time.Time:
		if v == nil {
			return nil
		}

		return *v
	case []string:
		out := make([]string, len(v))
		for i, s := range v {
			out[i] = str.Fold(s)
		}

		return out
	}

	return value
}

/*
numberOrTime reads a number as Unix seconds when a date condition asks.
*/
func numberOrTime(n *node, v float64) any {
	if dateOp(n.op) {
		return time.Unix(int64(v), 0)
	}

	return v
}

/*
parseTime reads an RFC 3339 timestamp or a calendar day, or keeps the
text so a wrong type simply fails to match.
*/
func parseTime(s string) any {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}

	if t, err := time.Parse(time.DateOnly, s); err == nil {
		return t
	}

	return s
}

/*
dateOp reports whether the operator compares dates.
*/
func dateOp(op string) bool {
	return valueShape(op) == valueDays || valueShape(op) == valueDate
}

/*
compare decides a condition on a normalized fact that is present.
*/
func compare(n *node, value any, env Env) bool {
	switch n.op {
	case OpExists:
		return true
	case OpNotExists:
		return false
	}

	switch v := value.(type) {
	case string:
		return compareText(n, v)
	case float64:
		return compareNumber(n, v)
	case bool:
		return compareBool(n, v)
	case time.Time:
		return compareDate(n, v, env)
	case []string:
		return compareSet(n, v)
	}

	return false
}

/*
compareText decides text and address conditions.
*/
func compareText(n *node, raw string) bool {
	if n.field.Type == TypeSet {
		return compareSet(n, []string{str.Fold(raw)})
	}

	v := str.Fold(raw)
	if n.field.Type == TypeID || n.field.Type == TypeSaved {
		v = raw
	}

	switch n.op {
	case OpEq:
		return isText(n) && v == first(n)
	case OpNeq:
		return isText(n) && v != first(n)
	case OpIn:
		return slices.Contains(n.operand.texts, v)
	case OpNotIn:
		return !slices.Contains(n.operand.texts, v)
	}

	return compareWithin(n, v, raw)
}

/*
first is the condition's single text value.
*/
func first(n *node) string {
	if len(n.operand.texts) == 0 {
		return ""
	}

	return n.operand.texts[0]
}

/*
compareWithin decides conditions on part of a text: substrings,
keywords and address patterns.
*/
func compareWithin(n *node, v, raw string) bool {
	want := first(n)

	switch n.op {
	case OpContains:
		return strings.Contains(v, want)
	case OpNotContains:
		return !strings.Contains(v, want)
	case OpStartsWith:
		return strings.HasPrefix(v, want)
	case OpEndsWith:
		return strings.HasSuffix(v, want)
	case OpContainsAny:
		return containsAnyWord(raw, n.operand.texts)
	case OpNotContainsAny:
		return !containsAnyWord(raw, n.operand.texts)
	case OpMatches:
		return urlmatch.Match(want, raw)
	}

	return false
}

/*
isText reports whether an equality compares text, so a text never
equals a number condition on a field of unknown type.
*/
func isText(n *node) bool {
	if n.operand.scalar == nil {
		return n.field.Type != TypeNumber
	}

	_, ok := n.operand.scalar.(string)

	return ok
}

/*
compareNumber decides number conditions.
*/
func compareNumber(n *node, v float64) bool {
	want, ok := n.operand.num, n.field.Type == TypeNumber
	if f, isNum := n.operand.scalar.(float64); isNum {
		want, ok = f, true
	}

	switch n.op {
	case OpEq:
		return ok && v == want
	case OpNeq:
		return ok && v != want
	case OpGt:
		return v > n.operand.num
	case OpGte:
		return v >= n.operand.num
	case OpLt:
		return v < n.operand.num
	case OpLte:
		return v <= n.operand.num
	case OpBetween:
		return v >= n.operand.lo && v <= n.operand.hi
	}

	return false
}

/*
compareBool decides boolean conditions.
*/
func compareBool(n *node, v bool) bool {
	switch n.op {
	case OpIsTrue:
		return v
	case OpIsFalse:
		return !v
	case OpEq, OpNeq:
		want, ok := n.operand.scalar.(bool)
		return ok && (v == want) == (n.op == OpEq)
	}

	return false
}

/*
compareSet decides tag-like conditions.
*/
func compareSet(n *node, v []string) bool {
	hasAny := slices.ContainsFunc(n.operand.texts, func(want string) bool { return slices.Contains(v, want) })
	switch n.op {
	case OpHas, OpHasAny:
		return hasAny
	case OpHasNone:
		return !hasAny
	}

	return false
}

/*
compareDate decides date conditions. A calendar day in a before, after
or on condition is a day in the workspace's location.
*/
func compareDate(n *node, v time.Time, env Env) bool {
	now, days := env.Now, time.Duration(n.operand.days)*day
	switch n.op {
	case OpBefore:
		return beforeDate(n.operand, v, env)
	case OpAfter:
		return afterDate(n.operand, v, env)
	case OpOn:
		return v.In(location(env)).Format(time.DateOnly) == onDay(n.operand, env)
	case OpWithinLastDays:
		return !v.After(now) && !v.Before(now.Add(-days))
	case OpMoreThanDaysAgo:
		return v.Before(now.Add(-days))
	case OpDaysAgo:
		return v.After(now.Add(-days-day)) && !v.After(now.Add(-days))
	case OpWithinNextDays:
		return !v.Before(now) && !v.After(now.Add(days))
	case OpMoreThanDaysFromNow:
		return v.After(now.Add(days))
	}

	return false
}

/*
dayStart is the moment a calendar day condition's day begins in the
workspace's location.
*/
func dayStart(o operand, env Env) time.Time {
	d, _ := time.ParseInLocation(time.DateOnly, o.day, location(env))
	return d
}

/*
beforeDate reports whether the time is before the given moment, or
before the given calendar day begins.
*/
func beforeDate(o operand, v time.Time, env Env) bool {
	if o.dayOnly {
		return v.Before(dayStart(o, env))
	}

	return v.Before(o.date)
}

/*
afterDate reports whether the time is after the given moment, or after
the whole given calendar day.
*/
func afterDate(o operand, v time.Time, env Env) bool {
	if o.dayOnly {
		return !v.Before(dayStart(o, env).AddDate(0, 0, 1))
	}

	return v.After(o.date)
}

/*
onDay is the calendar day an on condition names.
*/
func onDay(o operand, env Env) string {
	if o.dayOnly {
		return o.day
	}

	return o.date.In(location(env)).Format(time.DateOnly)
}

/*
location is the workspace's location, UTC when unset.
*/
func location(env Env) *time.Location {
	if env.Location == nil {
		return time.UTC
	}

	return env.Location
}

/*
containsAnyWord reports whether the text holds any keyword as whole
words, ignoring case and accents.
*/
func containsAnyWord(text string, keywords []string) bool {
	folded := str.FoldAccents(text)
	for _, kw := range keywords {
		if containsWord(folded, str.FoldAccents(kw)) {
			return true
		}
	}

	return false
}

/*
containsWord finds the keyword with no letter or digit right before or
after it.
*/
func containsWord(text, kw string) bool {
	if kw == "" {
		return false
	}

	for start := 0; start <= len(text)-len(kw); {
		i := strings.Index(text[start:], kw)
		if i < 0 {
			return false
		}

		at := start + i
		if boundary(text, at-1) && boundary(text, at+len(kw)) {
			return true
		}

		start = at + 1
	}

	return false
}

/*
boundary reports whether the byte position is outside a word.
*/
func boundary(text string, i int) bool {
	if i < 0 || i >= len(text) {
		return true
	}

	r := rune(text[i])
	if r >= 0x80 {
		return false
	}

	return !unicode.IsLetter(r) && !unicode.IsDigit(r)
}
