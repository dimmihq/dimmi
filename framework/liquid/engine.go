/*
|--------------------------------------------------------------------------
| Engine
|--------------------------------------------------------------------------
|
| An engine holds the filters templates may use and parses templates
| with them, so a filter it does not know is an error at parse time. The
| limits a render runs under live here too: steps, where every node, loop
| turn and filter costs one; a time; a size of output, captures included;
| and a depth of partials.
|
*/

package liquid

import (
	"errors"
	"maps"
	"sort"
	"time"
)

/*
Limits bound one render: Steps it may take, the Time it may run, the
bytes of Output it may write and the Depth partials may nest to.
*/
type Limits struct {
	Steps  int
	Time   time.Duration
	Output int
	Depth  int
}

/*
DefaultLimits returns limits generous for any real page that stop a
runaway one in well under a second.
*/
func DefaultLimits() Limits {
	return Limits{Steps: 2_000_000, Time: 2 * time.Second, Output: 8 << 20, Depth: 16}
}

/*
ErrLimit is wrapped by the error a render stops with when it runs out of
one of its limits.
*/
var ErrLimit = errors.New("render limit reached")

/*
Filter is a function a template pipes values through.
*/
type Filter func(c *Call) (any, error)

/*
Call is what a filter receives: the value piped in, its arguments in
order and by name, the data the render was given for filters, and Now,
the time the render started by its clock.
*/
type Call struct {
	Input any
	Args  []any
	Named map[string]any
	Data  any
	Now   time.Time
}

/*
Arg returns the argument at the index, nil when there is none.
*/
func (c *Call) Arg(i int) any {
	if i < len(c.Args) {
		return c.Args[i]
	}
	return nil
}

/*
Engine parses templates with its filters, Liquid's own and those added.
*/
type Engine struct {
	filters map[string]Filter
}

/*
New returns an engine with Liquid's standard filters.
*/
func New() *Engine {
	e := &Engine{filters: map[string]Filter{}}
	maps.Copy(e.filters, standardFilters())
	return e
}

/*
Filter adds a filter, or replaces one of the same name.
*/
func (e *Engine) Filter(name string, f Filter) {
	e.filters[name] = f
}

/*
Filters returns the names of the engine's filters, sorted.
*/
func (e *Engine) Filters() []string {
	names := make([]string, 0, len(e.filters))
	for name := range e.filters {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

/*
Template is a parsed template.
*/
type Template struct {
	name  string
	nodes []node
}

/*
Name returns the template's name.
*/
func (t *Template) Name() string { return t.name }

/*
Parse compiles one template.
*/
func (e *Engine) Parse(name, src string) (*Template, error) {
	nodes, err := parse(name, src, e.filters)
	if err != nil {
		return nil, err
	}
	return &Template{name: name, nodes: nodes}, nil
}
