/*
|--------------------------------------------------------------------------
| Rules
|--------------------------------------------------------------------------
|
| A branch's rule is decided in code, never by the model: the facts are
| the customer's trusted fields, the conversation's own, and what the
| actions already called returned. A rule that reads an action's result
| before the action ran is neither true nor false yet: it waits, and
| names the action that settles it, so dimmi is told to call it first.
|
| Fields are dotted paths: contact.email, contact.attr.plan,
| company.attr.tier, conversation.channel, and action.<tool>.<path>
| for a field of an action's result.
|
*/

package procedure

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
)

/*
ConditionExpr is a rule dimmi decides in code: all or any of its rules.
*/
type ConditionExpr struct {
	Mode  string          `json:"mode"`
	Rules []ConditionRule `json:"rules"`
}

/*
ConditionRule compares one field with a value.
*/
type ConditionRule struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    any    `json:"value,omitempty"`
}

/*
The operators a rule compares with.
*/
const (
	OpEq        = "eq"
	OpNeq       = "neq"
	OpGt        = "gt"
	OpGte       = "gte"
	OpLt        = "lt"
	OpLte       = "lte"
	OpIn        = "in"
	OpNotIn     = "not_in"
	OpContains  = "contains"
	OpExists    = "exists"
	OpNotExists = "not_exists"
)

/*
Operators lists every operator.
*/
func Operators() []string {
	return []string{OpEq, OpNeq, OpGt, OpGte, OpLt, OpLte, OpIn, OpNotIn, OpContains, OpExists, OpNotExists}
}

/*
actionPrefix starts a field read from an action's result.
*/
const actionPrefix = "action."

/*
Facts are what a rule reads: Lookup answers every field but an
action's, and Outputs holds each called action's result by tool name.
*/
type Facts struct {
	Lookup  func(field string) (any, bool)
	Outputs map[string]any
}

/*
Verdict is a rule's outcome: Known says whether it is decided, Value
what it decided, and Needs the tools whose results it waits for.
*/
type Verdict struct {
	Known bool
	Value bool
	Needs []string
}

/*
Evaluate decides a rule from the facts.
*/
func Evaluate(c ConditionExpr, f Facts) Verdict {
	var needs []string
	known := make([]bool, 0, len(c.Rules))
	for _, r := range c.Rules {
		value, wait := f.value(r.Field)
		if wait != "" {
			if !slices.Contains(needs, wait) {
				needs = append(needs, wait)
			}
			continue
		}
		known = append(known, compare(r.Operator, value, r.Value))
	}

	if c.Mode == "any" {
		if slices.Contains(known, true) {
			return Verdict{Known: true, Value: true}
		}
		if len(needs) > 0 {
			return Verdict{Needs: needs}
		}
		return Verdict{Known: true}
	}

	if slices.Contains(known, false) {
		return Verdict{Known: true}
	}
	if len(needs) > 0 {
		return Verdict{Needs: needs}
	}

	return Verdict{Known: true, Value: len(known) > 0}
}

/*
value reads one field, or names the tool it waits for.
*/
func (f Facts) value(field string) (any, string) {
	if rest, ok := strings.CutPrefix(field, actionPrefix); ok {
		tool, path, _ := strings.Cut(rest, ".")
		out, called := f.Outputs[tool]
		if !called {
			return nil, tool
		}
		if path == "" {
			return out, ""
		}
		return Dig(out, path), ""
	}

	if f.Lookup == nil {
		return nil, ""
	}

	v, ok := f.Lookup(field)
	if !ok {
		return nil, ""
	}

	return v, ""
}

/*
ActionField splits a field read from an action's result into the tool
and the path, false for any other field.
*/
func ActionField(field string) (string, string, bool) {
	rest, ok := strings.CutPrefix(field, actionPrefix)
	if !ok {
		return "", "", false
	}

	tool, path, _ := strings.Cut(rest, ".")

	return tool, path, true
}

/*
Dig reads a dotted path out of decoded JSON; a number in the path picks
an element of a list.
*/
func Dig(v any, path string) any {
	for part := range strings.SplitSeq(path, ".") {
		switch node := v.(type) {
		case map[string]any:
			v = node[part]
		case []any:
			i, err := strconv.Atoi(part)
			if err != nil || i < 0 || i >= len(node) {
				return nil
			}
			v = node[i]
		default:
			return nil
		}
	}

	return v
}

/*
compare applies one operator.
*/
func compare(op string, have, want any) bool {
	switch op {
	case OpExists:
		return present(have)
	case OpNotExists:
		return !present(have)
	case OpEq:
		return equal(have, want)
	case OpNeq:
		return !equal(have, want)
	case OpGt, OpGte, OpLt, OpLte:
		return ordered(op, have, want)
	case OpIn:
		return slices.ContainsFunc(listOf(want), func(w any) bool { return equal(have, w) })
	case OpNotIn:
		return present(have) && !slices.ContainsFunc(listOf(want), func(w any) bool { return equal(have, w) })
	case OpContains:
		return contains(have, want)
	}

	return false
}

/*
present reports whether a value is there: not nil, not an empty string
or list.
*/
func present(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(x) != ""
	case []any:
		return len(x) > 0
	case []string:
		return len(x) > 0
	}

	return true
}

/*
equal compares two values as numbers when both read as numbers, as
booleans when either is one, and otherwise as text without regard to
case or surrounding space.
*/
func equal(a, b any) bool {
	if x, ok := number(a); ok {
		if y, ok := number(b); ok {
			return math.Abs(x-y) < 1e-9
		}
	}

	if x, ok := a.(bool); ok {
		return strings.EqualFold(text(b), strconv.FormatBool(x))
	}

	if y, ok := b.(bool); ok {
		return strings.EqualFold(text(a), strconv.FormatBool(y))
	}

	if a == nil || b == nil {
		return a == nil && b == nil
	}

	return strings.EqualFold(strings.TrimSpace(text(a)), strings.TrimSpace(text(b)))
}

/*
ordered compares two numbers, or two moments.
*/
func ordered(op string, have, want any) bool {
	x, okx := number(have)
	y, oky := number(want)
	if !okx || !oky {
		tx, okx := moment(have)
		ty, oky := moment(want)
		if !okx || !oky {
			return false
		}
		x, y = float64(tx.UnixNano()), float64(ty.UnixNano())
	}

	switch op {
	case OpGt:
		return x > y
	case OpGte:
		return x >= y
	case OpLt:
		return x < y
	}

	return x <= y
}

/*
contains reports whether text holds a part, or a list holds a value.
*/
func contains(have, want any) bool {
	switch x := have.(type) {
	case []any:
		return slices.ContainsFunc(x, func(v any) bool { return equal(v, want) })
	case []string:
		return slices.ContainsFunc(x, func(v string) bool { return equal(v, want) })
	case nil:
		return false
	}

	return strings.Contains(strings.ToLower(text(have)), strings.ToLower(text(want)))
}

/*
number reads a value as a number.
*/
func number(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f, err == nil
	}

	return 0, false
}

/*
moment reads a value as a point in time.
*/
func moment(v any) (time.Time, bool) {
	switch x := v.(type) {
	case time.Time:
		return x, true
	case string:
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339, time.DateOnly} {
			if t, err := time.Parse(layout, strings.TrimSpace(x)); err == nil {
				return t, true
			}
		}
	}

	return time.Time{}, false
}

/*
listOf reads a rule's value as a list; a comma-separated text is one.
*/
func listOf(v any) []any {
	switch x := v.(type) {
	case []any:
		return x
	case []string:
		out := make([]any, len(x))
		for i, s := range x {
			out[i] = s
		}
		return out
	case string:
		var out []any
		for part := range strings.SplitSeq(x, ",") {
			if part = strings.TrimSpace(part); part != "" {
				out = append(out, part)
			}
		}
		return out
	}

	return []any{v}
}

/*
text writes any value as text.
*/
func text(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case nil:
		return ""
	}

	return fmt.Sprint(v)
}

/*
Describe writes a rule as a person reads it, for the model and the
trace.
*/
func Describe(c ConditionExpr) string {
	parts := make([]string, 0, len(c.Rules))
	for _, r := range c.Rules {
		phrase := r.Field + " " + strings.ReplaceAll(r.Operator, "_", " ")
		if r.Operator != OpExists && r.Operator != OpNotExists {
			v, _ := json.Marshal(r.Value)
			phrase += " " + string(v)
		}
		parts = append(parts, phrase)
	}

	join := " and "
	if c.Mode == "any" {
		join = " or "
	}

	return strings.Join(parts, join)
}
