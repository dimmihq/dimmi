/*
|--------------------------------------------------------------------------
| Render State
|--------------------------------------------------------------------------
|
| What a render keeps while it runs: the budget a page and its partials
| share, the variables in scope and where the output goes. Every step is
| counted, and every thousand or so the deadline and the caller's context
| are checked, so a render stops soon after either runs out.
|
*/

package liquid

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
)

/*
run is what a page and its partials share: the budget and the layout.
Stopped returns the caller's context error once it ends, and now is
when the render started.
*/
type run struct {
	stopped   func() error
	set       *Set
	limits    Limits
	globals   map[string]any
	clock     clock.Clock
	now       time.Time
	deadline  time.Time
	steps     int
	written   int
	depth     int
	layout    string
	layoutSet bool
}

/*
state renders one template: its locals, the loop scopes open in it and
where its output goes.
*/
type state struct {
	run      *run
	template string
	locals   map[string]any
	scopes   []map[string]any
	data     any
	out      io.Writer
}

/*
spend counts steps against the limit, and now and then checks the
deadline and the caller's context.
*/
func (s *state) spend(n int) error {
	r := s.run
	r.steps += n
	if r.steps > r.limits.Steps {
		return fmt.Errorf("%w: more than %d steps", ErrLimit, r.limits.Steps)
	}
	if r.steps&1023 >= n && n <= 1023 {
		return nil
	}
	if r.clock.Now().After(r.deadline) {
		return fmt.Errorf("%w: took longer than %s", ErrLimit, r.limits.Time)
	}
	return r.stopped()
}

/*
nodes renders nodes in order, a step each.
*/
func (s *state) nodes(nodes []node) error {
	for _, n := range nodes {
		if err := s.spend(1); err != nil {
			return &Error{Template: s.template, Message: err.Error(), Err: err}
		}
		if err := n.render(s); err != nil {
			return err
		}
	}
	return nil
}

/*
write writes text to the output.
*/
func (s *state) write(text string) error {
	if s.out == nil {
		s.out = &limited{w: io.Discard, run: s.run}
	}
	_, err := io.WriteString(s.out, text)
	return err
}

/*
capture renders nodes into text, counted against the output limit.
*/
func (s *state) capture(nodes []node) (string, error) {
	var b strings.Builder
	saved := s.out
	s.out = &limited{w: &b, run: s.run}
	err := s.nodes(nodes)
	s.out = saved
	return b.String(), err
}

/*
lookup reads a variable from the loop scopes, then the locals, then the
globals.
*/
func (s *state) lookup(name string) any {
	for _, v := range slices.Backward(s.scopes) {
		if v, ok := v[name]; ok {
			return v
		}
	}
	if v, ok := s.locals[name]; ok {
		return v
	}
	if v, ok := s.run.globals[name]; ok {
		return v
	}
	return nil
}

func (s *state) push(scope map[string]any) { s.scopes = append(s.scopes, scope) }

func (s *state) pop() { s.scopes = s.scopes[:len(s.scopes)-1] }

/*
partial renders the named partial with only the arguments and the
globals in its scope, as Liquid's render keeps a partial from reading
or changing the variables around it.
*/
func (s *state) partial(name string, args map[string]any, line int) error {
	if s.run.depth >= s.run.limits.Depth {
		message := fmt.Sprintf("partials nest deeper than %d", s.run.limits.Depth)
		return &Error{Template: s.template, Line: line, Message: message}
	}
	t, ok := s.run.set.partial(name)
	if !ok {
		return &Error{Template: s.template, Line: line, Message: fmt.Sprintf("no partial named %q", name)}
	}
	child := &state{run: s.run, template: t.name, locals: args, data: s.data, out: s.out}
	s.run.depth++
	defer func() { s.run.depth-- }()
	return child.nodes(t.nodes)
}

/*
limited writes until the render's output limit.
*/
type limited struct {
	w   io.Writer
	run *run
}

func (l *limited) Write(p []byte) (int, error) {
	l.run.written += len(p)
	if l.run.written > l.run.limits.Output {
		return 0, fmt.Errorf("%w: more than %d bytes of output", ErrLimit, l.run.limits.Output)
	}
	return l.w.Write(p)
}
