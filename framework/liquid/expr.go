/*
|--------------------------------------------------------------------------
| Expressions
|--------------------------------------------------------------------------
|
| What sits inside {{ }} and the arguments of tags: literals, variables
| with their dots and brackets, ranges such as (1..5), comparisons joined
| by and or, and filters piped one after another, each with its
| arguments, named ones included. Liquid has no arithmetic outside
| filters and no precedence between and or: they read from the right.
|
*/

package liquid

import (
	"fmt"
)

/*
expr is an expression a render evaluates.
*/
type expr interface {
	eval(s *state) (any, error)
}

type literal struct{ value any }

func (l literal) eval(*state) (any, error) { return l.value, nil }

/*
variable is a name with the steps into it: .key, [index] or ["key"].
*/
type variable struct {
	name  string
	steps []step
}

type step struct {
	key   string
	index expr
}

func (v variable) eval(s *state) (any, error) {
	value := s.lookup(v.name)
	for _, st := range v.steps {
		key := any(st.key)
		if st.index != nil {
			k, err := st.index.eval(s)
			if err != nil {
				return nil, err
			}
			key = k
		}
		value = property(value, key)
	}
	return value, nil
}

/*
property reads a key of a map, an index of a list, or the size, first
and last every list, map and text answers to.
*/
func property(value, key any) any {
	switch x := value.(type) {
	case map[string]any:
		k := toString(key)
		if v, ok := x[k]; ok {
			return v
		}
		if k == "size" {
			return len(x)
		}
	case []any:
		return listProperty(x, key)
	case string, HTML:
		if toString(key) == "size" {
			return size(x)
		}
	}
	return nil
}

/*
listProperty reads an index of a list, counting from the end when it is
negative, or its size, first or last.
*/
func listProperty(x []any, key any) any {
	if i, isNumber, ok := numeric(key); ok && isNumber {
		n := int(i)
		if n < 0 {
			n += len(x)
		}
		if n >= 0 && n < len(x) {
			return x[n]
		}
		return nil
	}
	switch toString(key) {
	case "size":
		return len(x)
	case "first":
		if len(x) > 0 {
			return x[0]
		}
	case "last":
		if len(x) > 0 {
			return x[len(x)-1]
		}
	}
	return nil
}

type rangeExpr struct{ from, to expr }

/*
maxRange is the longest range a template may write, so (1..1000000000)
never builds a list the size of the server's memory.
*/
const maxRange = 100000

func (r rangeExpr) eval(s *state) (any, error) {
	a, err := r.from.eval(s)
	if err != nil {
		return nil, err
	}
	b, err := r.to.eval(s)
	if err != nil {
		return nil, err
	}
	from, to := toInt(a), toInt(b)
	if to-from >= maxRange {
		return nil, fmt.Errorf("range (%d..%d) is longer than %d", from, to, maxRange)
	}
	out := make([]any, 0, max(0, to-from+1))
	for i := from; i <= to; i++ {
		out = append(out, i)
	}
	return out, s.spend(len(out))
}

type comparison struct {
	op          string
	left, right expr
}

func (c comparison) eval(s *state) (any, error) {
	a, err := c.left.eval(s)
	if err != nil {
		return nil, err
	}
	if c.op == "and" && !truthy(a) {
		return false, nil
	}
	if c.op == "or" && truthy(a) {
		return true, nil
	}
	b, err := c.right.eval(s)
	if err != nil {
		return nil, err
	}
	return compareWith(c.op, a, b), nil
}

/*
compareWith applies the operator to two values already evaluated.
Values that cannot be ordered compare false.
*/
func compareWith(op string, a, b any) bool {
	switch op {
	case "and", "or":
		return truthy(b)
	case "==":
		return equal(a, b)
	case "!=", "<>":
		return !equal(a, b)
	case "contains":
		return contains(a, b)
	}
	order, ok := compare(a, b)
	if !ok {
		return false
	}
	switch op {
	case "<":
		return order < 0
	case ">":
		return order > 0
	case "<=":
		return order <= 0
	case ">=":
		return order >= 0
	}
	return false
}

/*
filtered is a value piped through filters.
*/
type filtered struct {
	base    expr
	filters []filterCall
}

type filterCall struct {
	name  string
	fn    Filter
	args  []expr
	named []namedArg
}

type namedArg struct {
	name  string
	value expr
}

func (f filtered) eval(s *state) (any, error) {
	value, err := f.base.eval(s)
	if err != nil {
		return nil, err
	}
	for _, call := range f.filters {
		c, err := call.with(s, value)
		if err != nil {
			return nil, err
		}
		if err := s.spend(1); err != nil {
			return nil, err
		}
		value, err = call.fn(c)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", call.name, err)
		}
	}
	return value, nil
}

/*
with evaluates the call's arguments for the value piped in.
*/
func (fc filterCall) with(s *state, input any) (*Call, error) {
	c := &Call{Input: input, Data: s.data, Named: map[string]any{}, Now: s.run.now}
	for _, a := range fc.args {
		v, err := a.eval(s)
		if err != nil {
			return nil, err
		}
		c.Args = append(c.Args, v)
	}
	for _, n := range fc.named {
		v, err := n.value.eval(s)
		if err != nil {
			return nil, err
		}
		c.Named[n.name] = v
	}
	return c, nil
}
