/*
|--------------------------------------------------------------------------
| Conversation View Definitions
|--------------------------------------------------------------------------
|
| A saved view describes its conversations as groups of conditions: the
| groups joined by and or or, the conditions of each group likewise, two
| levels and no deeper, so anyone can read a view back. Every field and
| operator comes from a fixed registry; a definition naming anything else
| is refused with the path to the culprit, and values only ever reach the
| database as bound parameters, so a definition stays data whatever it
| holds.
|
*/

package querybuilder

import (
	"encoding/json"
	"fmt"
	"strings"
)

/*
Limits on a definition.
*/
const (
	MaxDefinitionGroups     = 5
	MaxDefinitionConditions = 20
)

/*
Definition is a saved view's conditions: groups joined by Op, and or
or.
*/
type Definition struct {
	Op     string  `json:"op"`
	Groups []Group `json:"groups"`
}

/*
Group is conditions joined by Op, and or or.
*/
type Group struct {
	Op         string      `json:"op"`
	Conditions []Condition `json:"conditions"`
}

/*
Condition is one test: a field, an operator and the value it compares
with, whose shape depends on both.
*/
type Condition struct {
	Field string          `json:"field"`
	Op    string          `json:"op"`
	Value json.RawMessage `json:"value,omitempty"`
}

/*
DefinitionError says where a definition is wrong: the path to the
group or condition, such as groups.1.conditions.0.op, and why.
*/
type DefinitionError struct {
	Path   string
	Reason string
}

/*
Error describes the mistake.
*/
func (e *DefinitionError) Error() string {
	return fmt.Sprintf("invalid definition at %s: %s", e.Path, e.Reason)
}

/*
The ways groups and conditions join.
*/
const (
	opAnd = "and"
	opOr  = "or"
)

/*
shape checks the definition's structure: the joins, and how many groups
and conditions it holds.
*/
func (d Definition) shape() error {
	if d.Op != opAnd && d.Op != opOr {
		return &DefinitionError{Path: "op", Reason: `must be "and" or "or"`}
	}

	if len(d.Groups) == 0 || len(d.Groups) > MaxDefinitionGroups {
		return &DefinitionError{Path: "groups", Reason: fmt.Sprintf("between 1 and %d groups", MaxDefinitionGroups)}
	}

	total := 0

	for i, g := range d.Groups {
		if g.Op != opAnd && g.Op != opOr {
			return &DefinitionError{Path: fmt.Sprintf("groups.%d.op", i), Reason: `must be "and" or "or"`}
		}

		if len(g.Conditions) == 0 {
			return &DefinitionError{Path: fmt.Sprintf("groups.%d.conditions", i), Reason: "at least one condition"}
		}

		total += len(g.Conditions)
	}

	if total > MaxDefinitionConditions {
		return &DefinitionError{
			Path: "groups", Reason: fmt.Sprintf("at most %d conditions in all", MaxDefinitionConditions),
		}
	}

	return nil
}

/*
compiled is a predicate with its values, each placeholder a question
mark numbered only once the whole definition is put together.
*/
type compiled struct {
	sql  string
	args []any
}

/*
join puts the parts together with the operator.
*/
func join(op string, parts []compiled) compiled {
	sqls := make([]string, len(parts))

	var args []any

	for i, p := range parts {
		sqls[i] = "(" + p.sql + ")"
		args = append(args, p.args...)
	}

	return compiled{sql: strings.Join(sqls, " "+strings.ToUpper(op)+" "), args: args}
}

/*
fragment numbers the placeholders from the list's next free one.
*/
func (p compiled) fragment() Fragment {
	return func(next int) (string, []any) {
		var b strings.Builder

		n := next

		for _, r := range p.sql {
			if r != '?' {
				b.WriteRune(r)

				continue
			}

			fmt.Fprintf(&b, "$%d", n)
			n++
		}

		return "(" + b.String() + ")", p.args
	}
}
