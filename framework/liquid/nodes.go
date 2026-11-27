/*
|--------------------------------------------------------------------------
| Rendering Nodes
|--------------------------------------------------------------------------
|
| How each parsed node renders: text as it is, output escaped unless it
| is HTML, and every tag by its own rules. Loops spend a step a turn, and
| break and continue travel up as errors to the loop that catches them.
|
*/

package liquid

import (
	"errors"
	"maps"
	"slices"
)

/*
errorAt names the line a render went wrong on, once.
*/
func errorAt(s *state, line int, err error) error {
	var known *Error
	if errors.As(err, &known) || errors.Is(err, errBreak) || errors.Is(err, errContinue) {
		return err
	}
	return &Error{Template: s.template, Line: line, Message: err.Error(), Err: err}
}

func (n textNode) render(s *state) error {
	return s.write(n.text)
}

func (n outputNode) render(s *state) error {
	v, err := n.value.eval(s)
	if err != nil {
		return errorAt(s, n.line, err)
	}
	return s.write(printed(v))
}

func (n blockNode) render(s *state) error { return s.nodes(n) }

func (n ifNode) render(s *state) error {
	for _, br := range n.branches {
		v, err := br.test.eval(s)
		if err != nil {
			return err
		}
		if truthy(v) {
			return s.nodes(br.body)
		}
	}
	return s.nodes(n.otherwise)
}

func (n caseNode) render(s *state) error {
	subject, err := n.subject.eval(s)
	if err != nil {
		return err
	}
	for _, w := range n.whens {
		for _, e := range w.values {
			v, err := e.eval(s)
			if err != nil {
				return err
			}
			if equal(subject, v) {
				return s.nodes(w.body)
			}
		}
	}
	return s.nodes(n.otherwise)
}

func (n forNode) render(s *state) error {
	items, err := n.items(s)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return s.nodes(n.otherwise)
	}

	parent := s.lookup("forloop")
	loop := map[string]any{"length": len(items), "parentloop": parent}
	s.push(map[string]any{"forloop": loop})
	defer s.pop()
	for i, item := range items {
		if err := s.spend(1); err != nil {
			return err
		}
		loop["index"], loop["index0"] = i+1, i
		loop["rindex"], loop["rindex0"] = len(items)-i, len(items)-i-1
		loop["first"], loop["last"] = i == 0, i == len(items)-1
		s.scopes[len(s.scopes)-1][n.variable] = item
		err := s.nodes(n.body)
		if errors.Is(err, errBreak) {
			break
		}
		if err != nil && !errors.Is(err, errContinue) {
			return err
		}
	}
	return nil
}

/*
items is the list the loop runs over, after its offset, limit and
reversed.
*/
func (n forNode) items(s *state) ([]any, error) {
	source, err := n.source.eval(s)
	if err != nil {
		return nil, err
	}
	items := list(source)
	if n.offset != nil {
		o, err := n.offset.eval(s)
		if err != nil {
			return nil, err
		}
		items = items[min(max(0, toInt(o)), len(items)):]
	}
	if n.limit != nil {
		l, err := n.limit.eval(s)
		if err != nil {
			return nil, err
		}
		items = items[:min(max(0, toInt(l)), len(items))]
	}
	if n.reversed {
		items = slices.Clone(items)
		slices.Reverse(items)
	}
	return items, nil
}

func (n assignNode) render(s *state) error {
	v, err := n.value.eval(s)
	if err != nil {
		return err
	}
	s.locals[n.name] = v
	return nil
}

func (n captureNode) render(s *state) error {
	text, err := s.capture(n.body)
	if err != nil {
		return err
	}
	s.locals[n.name] = HTML(text)
	return nil
}

func (n layoutNode) render(s *state) error {
	s.run.layout, s.run.layoutSet = n.name, true
	return nil
}

func (n renderNode) render(s *state) error {
	args := map[string]any{}
	for _, a := range n.args {
		v, err := a.value.eval(s)
		if err != nil {
			return err
		}
		args[a.name] = v
	}
	if n.with == nil {
		return s.partial(n.partial, args, n.line)
	}
	v, err := n.with.eval(s)
	if err != nil {
		return err
	}
	if !n.loop {
		args[n.alias] = v
		return s.partial(n.partial, args, n.line)
	}
	items := list(v)
	for i, item := range items {
		each := make(map[string]any, len(args)+2)
		maps.Copy(each, args)
		each[n.alias] = item
		each["forloop"] = map[string]any{
			"index": i + 1, "index0": i, "first": i == 0, "last": i == len(items)-1, "length": len(items),
			"rindex": len(items) - i, "rindex0": len(items) - i - 1,
		}
		if err := s.partial(n.partial, each, n.line); err != nil {
			return err
		}
	}
	return nil
}
