/*
|--------------------------------------------------------------------------
| Tags
|--------------------------------------------------------------------------
|
| The tags a template may use, each parsed into a node that renders
| itself: if, unless and case choose, for loops with limit, offset,
| reversed and else, assign and capture name values, echo prints, render
| draws a partial in its own scope, and layout picks the frame a page
| sits in. Anything else is an error when the template is parsed.
|
*/

package liquid

import (
	"errors"
	"fmt"
	"strings"
)

var (
	errBreak    = errors.New("break")
	errContinue = errors.New("continue")
)

/*
tag parses the tag with the name into its node.
*/
func (b *builder) tag(name, args string, line int) (node, error) {
	switch name {
	case "if", "unless":
		return b.ifTag(name, args, line)
	case "case":
		return b.caseTag(args, line)
	case "for":
		return b.forTag(args, line)
	case "break":
		return controlNode{errBreak}, nil
	case "continue":
		return controlNode{errContinue}, nil
	case "assign":
		return b.assignTag(args, line)
	case "capture":
		return b.captureTag(args, line)
	case "echo":
		return b.echoTag(args, line)
	case "render":
		return b.renderTag(args, line)
	case "layout":
		return b.layoutTag(args, line)
	case "raw":
		return b.rawTag()
	case "comment":
		_, _, err := b.block("endcomment")
		return nil, err
	}
	return nil, b.fail(line, "unknown tag %q", name)
}

/*
outputExpr parses a whole value with its filters.
*/
func (b *builder) outputExpr(src string, line int) (expr, error) {
	return b.whole(src, line, (*parser).output)
}

/*
conditionExpr parses a whole condition.
*/
func (b *builder) conditionExpr(src string, line int) (expr, error) {
	return b.whole(src, line, (*parser).condition)
}

/*
whole parses an expression with read, failing when anything is left
after it.
*/
func (b *builder) whole(src string, line int, read func(*parser) (expr, error)) (expr, error) {
	p, err := newParser(src, b.filters)
	if err != nil {
		return nil, b.fail(line, "%v", err)
	}
	e, err := read(p)
	if err == nil {
		err = p.done()
	}
	if err != nil {
		return nil, b.fail(line, "%v", err)
	}
	return e, nil
}

/*
echoTag reads echo, which prints a value as {{ }} does.
*/
func (b *builder) echoTag(args string, line int) (node, error) {
	value, err := b.outputExpr(args, line)
	if err != nil {
		return nil, err
	}
	return outputNode{value: value, line: line}, nil
}

/*
rawTag keeps the text up to endraw as it is.
*/
func (b *builder) rawTag() (node, error) {
	body, _, err := b.block("endraw")
	if err != nil {
		return nil, err
	}
	return blockNode(body), nil
}

type blockNode []node

type controlNode struct{ signal error }

func (c controlNode) render(*state) error { return c.signal }

type branch struct {
	test expr
	body []node
}

type ifNode struct {
	branches  []branch
	otherwise []node
}

func (b *builder) ifTag(name, args string, line int) (node, error) {
	test, err := b.conditionExpr(args, line)
	if err != nil {
		return nil, err
	}
	if name == "unless" {
		test = notExpr{test}
	}
	end := "end" + name
	n := ifNode{}
	for {
		body, ended, err := b.block("elsif", "else", end)
		if err != nil {
			return nil, err
		}
		n.branches = append(n.branches, branch{test, body})
		switch ended.tag {
		case "elsif":
			test, err = b.conditionExpr(ended.args, ended.line)
			if err != nil {
				return nil, err
			}
			continue
		case "else":
			n.otherwise, _, err = b.block(end)
			if err != nil {
				return nil, err
			}
		}
		return n, nil
	}
}

type notExpr struct{ inner expr }

func (n notExpr) eval(s *state) (any, error) {
	v, err := n.inner.eval(s)
	return !truthy(v), err
}

type caseNode struct {
	subject   expr
	whens     []caseWhen
	otherwise []node
}

type caseWhen struct {
	values []expr
	body   []node
}

func (b *builder) caseTag(args string, line int) (node, error) {
	subject, err := b.outputExpr(args, line)
	if err != nil {
		return nil, err
	}
	n := caseNode{subject: subject}
	_, ended, err := b.block("when", "else", "endcase")
	if err != nil {
		return nil, err
	}
	for ended.tag == "when" {
		var values []expr
		for _, part := range splitWhen(ended.args) {
			v, err := b.conditionExpr(part, ended.line)
			if err != nil {
				return nil, err
			}
			values = append(values, v)
		}
		var body []node
		body, ended, err = b.block("when", "else", "endcase")
		if err != nil {
			return nil, err
		}
		n.whens = append(n.whens, caseWhen{values, body})
	}
	if ended.tag == "else" {
		n.otherwise, _, err = b.block("endcase")
		if err != nil {
			return nil, err
		}
	}
	return n, nil
}

/*
splitWhen cuts the values of a when at commas and the word or, outside
quotes.
*/
func splitWhen(src string) []string {
	var parts []string
	var cur strings.Builder
	quote := byte(0)
	flush := func() {
		if s := strings.TrimSpace(cur.String()); s != "" {
			parts = append(parts, s)
		}
		cur.Reset()
	}
	for i := 0; i < len(src); i++ {
		c := src[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
			cur.WriteByte(c)
		case c == '\'' || c == '"':
			quote = c
			cur.WriteByte(c)
		case c == ',':
			flush()
		case strings.HasPrefix(src[i:], " or "):
			flush()
			i += 3
		default:
			cur.WriteByte(c)
		}
	}
	flush()
	return parts
}

type forNode struct {
	variable  string
	source    expr
	limit     expr
	offset    expr
	reversed  bool
	body      []node
	otherwise []node
}

/*
forTag reads for item in list with its options, its body and its else.
*/
func (b *builder) forTag(args string, line int) (node, error) {
	variable, rest, ok := strings.Cut(args, " in ")
	if !ok {
		return nil, b.fail(line, "for needs the form: for item in list")
	}
	n := forNode{variable: strings.TrimSpace(variable)}
	if err := forOptions(&n, rest, b.filters); err != nil {
		return nil, b.fail(line, "%v", err)
	}
	body, ended, err := b.block("else", "endfor")
	if err != nil {
		return nil, err
	}
	n.body = body
	if ended.tag == "else" {
		if n.otherwise, _, err = b.block("endfor"); err != nil {
			return nil, err
		}
	}
	return n, nil
}

/*
forOptions reads what follows in: the list, then reversed, limit and
offset in any order.
*/
func forOptions(n *forNode, src string, filters map[string]Filter) error {
	p, err := newParser(src, filters)
	if err != nil {
		return err
	}
	if n.source, err = p.primary(); err != nil {
		return err
	}
	for p.peek().kind != tokEOF {
		if err := forOption(n, p); err != nil {
			return err
		}
	}
	return nil
}

/*
forOption reads one option of a for tag.
*/
func forOption(n *forNode, p *parser) error {
	t := p.next()
	switch {
	case t.kind == tokIdent && t.text == "reversed":
		n.reversed = true
	case t.kind == tokIdent && (t.text == "limit" || t.text == "offset"):
		if err := p.expect(":"); err != nil {
			return err
		}
		v, err := p.primary()
		if err != nil {
			return err
		}
		if t.text == "limit" {
			n.limit = v
		} else {
			n.offset = v
		}
	case t.kind == tokPunct && t.text == ",":
	default:
		return fmt.Errorf("unexpected %q in for", t.text)
	}
	return nil
}

type assignNode struct {
	name  string
	value expr
}

func (b *builder) assignTag(args string, line int) (node, error) {
	name, value, ok := strings.Cut(args, "=")
	name = strings.TrimSpace(name)
	if !ok || name == "" {
		return nil, b.fail(line, "assign needs the form: assign name = value")
	}
	e, err := b.outputExpr(value, line)
	if err != nil {
		return nil, err
	}
	return assignNode{name: name, value: e}, nil
}

type captureNode struct {
	name string
	body []node
}

func (b *builder) captureTag(args string, line int) (node, error) {
	name := strings.Trim(strings.TrimSpace(args), `'"`)
	if name == "" {
		return nil, b.fail(line, "capture needs a name")
	}
	body, _, err := b.block("endcapture")
	if err != nil {
		return nil, err
	}
	return captureNode{name: name, body: body}, nil
}

type renderNode struct {
	partial string
	with    expr
	loop    bool
	alias   string
	args    []namedArg
	line    int
}

/*
renderTag reads render 'name', with value as name or for list as name,
then named arguments.
*/
func (b *builder) renderTag(args string, line int) (node, error) {
	p, err := newParser(args, b.filters)
	if err != nil {
		return nil, b.fail(line, "%v", err)
	}
	n, err := renderParts(p)
	if err != nil {
		return nil, b.fail(line, "%v", err)
	}
	n.line = line
	return n, nil
}

/*
renderParts reads the partial's name, its with or for, and its
arguments.
*/
func renderParts(p *parser) (renderNode, error) {
	t := p.next()
	if t.kind != tokString {
		return renderNode{}, errors.New("render needs a quoted partial name")
	}
	n := renderNode{partial: t.text, alias: t.text}
	if i := strings.LastIndexByte(n.alias, '/'); i >= 0 {
		n.alias = n.alias[i+1:]
	}
	if err := renderWith(p, &n); err != nil {
		return renderNode{}, err
	}
	if err := renderArgs(p, &n); err != nil {
		return renderNode{}, err
	}
	return n, p.done()
}

/*
renderWith reads with value or for list, and the name after as.
*/
func renderWith(p *parser, n *renderNode) error {
	w := p.peek()
	if w.kind != tokIdent || (w.text != "with" && w.text != "for") {
		return nil
	}
	p.next()
	n.loop = w.text == "for"
	var err error
	if n.with, err = p.primary(); err != nil {
		return err
	}
	if !p.accept(tokIdent, "as") {
		return nil
	}
	a := p.next()
	if a.kind != tokIdent {
		return errors.New("expected a name after as")
	}
	n.alias = a.text
	return nil
}

/*
renderArgs reads the named arguments a partial receives.
*/
func renderArgs(p *parser, n *renderNode) error {
	for p.accept(tokPunct, ",") || p.peek().kind == tokIdent {
		key := p.next()
		if key.kind != tokIdent {
			return fmt.Errorf("expected an argument name, found %q", key.text)
		}
		if err := p.expect(":"); err != nil {
			return err
		}
		v, err := p.primary()
		if err != nil {
			return err
		}
		n.args = append(n.args, namedArg{key.text, v})
	}
	return nil
}

type layoutNode struct{ name string }

func (b *builder) layoutTag(args string, line int) (node, error) {
	name := strings.TrimSpace(args)
	if name == "none" {
		return layoutNode{name: ""}, nil
	}
	unquoted := strings.Trim(name, `'"`)
	if unquoted == name || unquoted == "" {
		return nil, b.fail(line, "layout needs a quoted name or none")
	}
	return layoutNode{name: unquoted}, nil
}
