/*
|--------------------------------------------------------------------------
| Sets
|--------------------------------------------------------------------------
|
| A set is every template of one theme, parsed once, which falls back to
| another set for a template or partial it lacks. A render runs under
| limits, all shared by the page and the partials it draws. Whichever
| runs out first stops the render with an error naming it.
|
*/

package liquid

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/dimmihq/dimmi/framework/clock"
)

/*
Set is the templates of one theme by name, with the set to fall back to
for a name it lacks. Partials resolves a partial's name to the template
holding it, such as "header" to "partials/header.liquid".
*/
type Set struct {
	templates map[string]*Template
	fallback  *Set
	partials  func(name string) string
}

/*
ParseSet compiles every template of a theme, returning all the errors
it found, not only the first, so a theme shows every mistake at once.
*/
func (e *Engine) ParseSet(sources map[string]string, partials func(string) string, fallback *Set) (*Set, []error) {
	set := &Set{templates: map[string]*Template{}, fallback: fallback, partials: partials}
	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	sort.Strings(names)
	var errs []error
	for _, name := range names {
		t, err := e.Parse(name, sources[name])
		if err != nil {
			errs = append(errs, err)
			continue
		}
		set.templates[name] = t
	}
	return set, errs
}

/*
Lookup finds the template with the name in the set or its fallback.
*/
func (s *Set) Lookup(name string) (*Template, bool) {
	for at := s; at != nil; at = at.fallback {
		if t, ok := at.templates[name]; ok {
			return t, true
		}
	}
	return nil, false
}

/*
Has says whether the set itself, not its fallback, holds the template.
*/
func (s *Set) Has(name string) bool {
	_, ok := s.templates[name]
	return ok
}

/*
partial finds a partial by the name a render tag gives it.
*/
func (s *Set) partial(name string) (*Template, bool) {
	if s.partials != nil {
		name = s.partials(name)
	}
	return s.Lookup(name)
}

/*
Options are what one render reads: the globals every template and
partial sees, the data filters receive, the limits it runs under
(DefaultLimits when zero), the locals already set, such as a layout
reading the variables its page assigned, and the clock it tells time
by (the system's when nil). The render writes the page's assigns into
Locals, a fresh map when nil.
*/
type Options struct {
	Globals map[string]any
	Data    any
	Limits  Limits
	Locals  map[string]any
	Clock   clock.Clock
}

/*
Result is what a page's render leaves besides its output: the layout it
asked for with the layout tag, if it used it, and the variables it
assigned, which its layout reads.
*/
type Result struct {
	Layout    string
	LayoutSet bool
	Assigned  map[string]any
}

/*
Render renders the named template of the set into the writer.
*/
func (s *Set) Render(ctx context.Context, w io.Writer, name string, opts Options) (Result, error) {
	t, ok := s.Lookup(name)
	if !ok {
		return Result{}, fmt.Errorf("liquid: no template named %q", name)
	}
	return s.RenderTemplate(ctx, w, t, opts)
}

/*
RenderTemplate renders a template of the set, or one parsed apart from
it, with the partials of the set.
*/
func (s *Set) RenderTemplate(ctx context.Context, w io.Writer, t *Template, opts Options) (Result, error) {
	limits := opts.Limits
	if limits == (Limits{}) {
		limits = DefaultLimits()
	}
	clk := opts.Clock
	if clk == nil {
		clk = clock.System()
	}
	now := clk.Now()
	run := &run{
		stopped: ctx.Err, set: s, limits: limits, globals: opts.Globals,
		clock: clk, now: now, deadline: now.Add(limits.Time),
	}
	locals := opts.Locals
	if locals == nil {
		locals = map[string]any{}
	}
	st := &state{run: run, template: t.name, locals: locals, data: opts.Data, out: &limited{w: w, run: run}}
	err := st.nodes(t.nodes)
	if errors.Is(err, errBreak) || errors.Is(err, errContinue) {
		err = nil
	}
	return Result{Layout: run.layout, LayoutSet: run.layoutSet, Assigned: locals}, err
}
