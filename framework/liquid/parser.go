/*
|--------------------------------------------------------------------------
| Expression Parser
|--------------------------------------------------------------------------
|
| The parser reads tokens into expressions. Filters are looked up as they
| are read, so a template naming one the engine lacks fails to parse.
|
*/

package liquid

import (
	"fmt"
	"strconv"
)

/*
parser reads tokens into expressions, checking filters against the
engine's.
*/
type parser struct {
	tokens  []token
	at      int
	filters map[string]Filter
}

func newParser(src string, filters map[string]Filter) (*parser, error) {
	tokens, err := lexExpr(src)
	if err != nil {
		return nil, err
	}
	return &parser{tokens: tokens, filters: filters}, nil
}

func (p *parser) peek() token { return p.tokens[p.at] }

func (p *parser) next() token {
	t := p.tokens[p.at]
	if t.kind != tokEOF {
		p.at++
	}
	return t
}

func (p *parser) accept(kind tokenKind, text string) bool {
	t := p.peek()
	if t.kind == kind && t.text == text {
		p.at++
		return true
	}
	return false
}

/*
expect reads the punctuation or fails.
*/
func (p *parser) expect(punct string) error {
	if !p.accept(tokPunct, punct) {
		return fmt.Errorf("expected %q, found %q", punct, p.peek().text)
	}
	return nil
}

func (p *parser) done() error {
	if t := p.peek(); t.kind != tokEOF {
		return fmt.Errorf("unexpected %q", t.text)
	}
	return nil
}

/*
condition reads comparisons joined by and or, right to left.
*/
func (p *parser) condition() (expr, error) {
	left, err := p.comparison()
	if err != nil {
		return nil, err
	}
	t := p.peek()
	if t.kind == tokIdent && (t.text == "and" || t.text == "or") {
		p.next()
		right, err := p.condition()
		if err != nil {
			return nil, err
		}
		return comparison{op: t.text, left: left, right: right}, nil
	}
	return left, nil
}

func (p *parser) comparison() (expr, error) {
	left, err := p.primary()
	if err != nil {
		return nil, err
	}
	t := p.peek()
	if t.kind == tokOp || (t.kind == tokIdent && t.text == "contains") {
		p.next()
		right, err := p.primary()
		if err != nil {
			return nil, err
		}
		return comparison{op: t.text, left: left, right: right}, nil
	}
	return left, nil
}

/*
primary reads a literal, a range or a variable.
*/
func (p *parser) primary() (expr, error) {
	t := p.next()
	switch t.kind {
	case tokString:
		return literal{t.text}, nil
	case tokNumber:
		return numberLiteral(t.text)
	case tokPunct:
		return p.punctPrimary(t.text)
	case tokIdent:
		return p.identPrimary(t.text)
	case tokEOF:
		return nil, fmt.Errorf("expected a value")
	case tokOp:
	}
	return nil, fmt.Errorf("unexpected %q", t.text)
}

/*
numberLiteral reads a whole number, or else a fraction.
*/
func numberLiteral(text string) (expr, error) {
	if i, err := strconv.Atoi(text); err == nil {
		return literal{i}, nil
	}
	f, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return nil, fmt.Errorf("bad number %q", text)
	}
	return literal{f}, nil
}

/*
punctPrimary reads what starts with punctuation: a range such as
(1..5), or a variable with a quoted name such as ["my key"].
*/
func (p *parser) punctPrimary(text string) (expr, error) {
	switch text {
	case "(":
		from, err := p.primary()
		if err != nil {
			return nil, err
		}
		if err := p.expect(".."); err != nil {
			return nil, err
		}
		to, err := p.primary()
		if err != nil {
			return nil, err
		}
		if err := p.expect(")"); err != nil {
			return nil, err
		}
		return rangeExpr{from, to}, nil
	case "[":
		key, err := p.primary()
		if err != nil {
			return nil, err
		}
		if err := p.expect("]"); err != nil {
			return nil, err
		}
		k, ok := key.(literal)
		if !ok {
			return nil, fmt.Errorf("a variable may start with a quoted name only")
		}
		return p.steps(variable{name: toString(k.value)})
	}
	return nil, fmt.Errorf("unexpected %q", text)
}

/*
identPrimary reads a keyword such as true or empty, or a variable.
*/
func (p *parser) identPrimary(text string) (expr, error) {
	switch text {
	case "true":
		return literal{true}, nil
	case "false":
		return literal{false}, nil
	case "nil", "null":
		return literal{nil}, nil
	case "empty":
		return literal{emptyValue{}}, nil
	case "blank":
		return literal{blankValue{}}, nil
	}
	return p.steps(variable{name: text})
}

/*
steps reads the dots and brackets after a variable's name.
*/
func (p *parser) steps(v variable) (expr, error) {
	for {
		switch {
		case p.accept(tokPunct, "."):
			t := p.next()
			if t.kind != tokIdent {
				return nil, fmt.Errorf("expected a name after the dot, found %q", t.text)
			}
			v.steps = append(v.steps, step{key: t.text})
		case p.accept(tokPunct, "["):
			st, err := p.indexStep()
			if err != nil {
				return nil, err
			}
			v.steps = append(v.steps, st)
		default:
			return v, nil
		}
	}
}

/*
indexStep reads what sits inside brackets: a quoted key, kept as a key,
or any other value, evaluated as the render reaches it.
*/
func (p *parser) indexStep() (step, error) {
	index, err := p.primary()
	if err != nil {
		return step{}, err
	}
	if err := p.expect("]"); err != nil {
		return step{}, err
	}
	if l, ok := index.(literal); ok {
		if s, isText := l.value.(string); isText {
			return step{key: s}, nil
		}
	}
	return step{index: index}, nil
}

/*
output reads a value with its filters, as {{ }} and assign hold.
*/
func (p *parser) output() (expr, error) {
	base, err := p.primary()
	if err != nil {
		return nil, err
	}
	var calls []filterCall
	for p.accept(tokPunct, "|") {
		call, err := p.filter()
		if err != nil {
			return nil, err
		}
		calls = append(calls, call)
	}
	if len(calls) == 0 {
		return base, nil
	}
	return filtered{base: base, filters: calls}, nil
}

/*
filter reads one filter after its pipe: the name, then its arguments
after a colon, split by commas.
*/
func (p *parser) filter() (filterCall, error) {
	name := p.next()
	if name.kind != tokIdent {
		return filterCall{}, fmt.Errorf("expected a filter name, found %q", name.text)
	}
	fn, ok := p.filters[name.text]
	if !ok {
		return filterCall{}, fmt.Errorf("unknown filter %q", name.text)
	}
	call := filterCall{name: name.text, fn: fn}
	if !p.accept(tokPunct, ":") {
		return call, nil
	}
	for {
		if err := p.filterArg(&call); err != nil {
			return filterCall{}, err
		}
		if !p.accept(tokPunct, ",") {
			return call, nil
		}
	}
}

/*
filterArg reads one argument of a filter, named when a name and a colon
come first.
*/
func (p *parser) filterArg(call *filterCall) error {
	name := ""
	if p.namedNext() {
		name = p.next().text
		p.next()
	}
	value, err := p.primary()
	if err != nil {
		return err
	}
	if name == "" {
		call.args = append(call.args, value)
	} else {
		call.named = append(call.named, namedArg{name, value})
	}
	return nil
}

/*
namedNext says whether a name and a colon come next.
*/
func (p *parser) namedNext() bool {
	if p.peek().kind != tokIdent {
		return false
	}
	after := p.tokens[p.at+1]
	return after.kind == tokPunct && after.text == ":"
}
