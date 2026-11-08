/*
|--------------------------------------------------------------------------
| Messenger Rules
|--------------------------------------------------------------------------
|
| A messenger may limit who sees its launcher, who may start a
| conversation and who sees each tab. Every rule is decided on the
| server with everything it knows about the visitor. A browser is told
| only the outcome: yes, no, or the page and device conditions still to
| check on each page. Attribute names and values, saved audiences and
| event names never leave the server. No rule means everyone.
|
*/

package audiences

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/dimmihq/dimmi/app/support/audience"
)

/*
messengerTabs are the tabs a messenger rule may hide.
*/
func messengerTabs() []string {
	return []string{"help", "changelog"}
}

/*
MessengerRules are a messenger's stored rules. Tabs holds a rule per
tab name.
*/
type MessengerRules struct {
	Launcher *audience.Audience            `json:"launcher,omitempty"`
	Start    *audience.Audience            `json:"start,omitempty"`
	Tabs     map[string]*audience.Audience `json:"tabs,omitempty"`
}

/*
Messenger is a messenger's compiled rules; a nil program means everyone.
*/
type Messenger struct {
	Launcher *audience.Program
	Start    *audience.Program
	Tabs     map[string]*audience.Program
}

/*
Surface is what a browser is told. Launcher and each tab are true, false
or the rule still to check on each page. Start says whether this visitor
may start a conversation.
*/
type Surface struct {
	Launcher any            `json:"launcher"`
	Start    bool           `json:"start"`
	Tabs     map[string]any `json:"tabs,omitempty"`
}

/*
ParseMessenger reads stored rules, empty meaning none.
*/
func ParseMessenger(raw json.RawMessage) (MessengerRules, error) {
	var r MessengerRules
	if len(raw) == 0 {
		return r, nil
	}

	if err := json.Unmarshal(raw, &r); err != nil {
		return r, fmt.Errorf("read messenger rules: %w", err)
	}

	return r, nil
}

/*
Empty reports whether no rule limits the messenger.
*/
func (r MessengerRules) Empty() bool {
	return r.Launcher == nil && r.Start == nil && len(r.Tabs) == 0
}

/*
CompileMessenger compiles every rule against its surface, listing every
problem with the rule's path.
*/
func CompileMessenger(r MessengerRules) (Messenger, []audience.Problem) {
	out := Messenger{Tabs: map[string]*audience.Program{}}
	var found []audience.Problem

	compile := func(path string, a *audience.Audience, s audience.Surface) *audience.Program {
		if a == nil {
			return nil
		}

		p, err := audience.Compile(*a, audience.Base(), s)
		found = append(found, prefixed(path, err)...)

		return p
	}

	out.Launcher = compile("launcher", r.Launcher, audience.MessengerDisplay("launcher"))
	out.Start = compile("start", r.Start, audience.MessengerStart())

	for _, tab := range slices.Sorted(maps.Keys(r.Tabs)) {
		if !slices.Contains(messengerTabs(), tab) {
			found = append(found, audience.Problem{
				Code: audience.CodeSchema, Path: "tabs." + tab,
				Message: "Only the help and changelog tabs take a rule.",
			})

			continue
		}

		out.Tabs[tab] = compile("tabs."+tab, r.Tabs[tab], audience.MessengerDisplay("tab."+tab))
	}

	return out, found
}

/*
prefixed puts a rule's path before the paths of its problems.
*/
func prefixed(path string, err error) []audience.Problem {
	var problems *audience.ProblemsError
	if err == nil || !asProblems(err, &problems) {
		return nil
	}

	out := make([]audience.Problem, 0, len(problems.Problems))
	for _, p := range problems.Problems {
		p.Path = path + "." + p.Path
		out = append(out, p)
	}

	return out
}

/*
Decide tells what a visitor gets, with everything the environment
knows. Start is only ever yes when the rule is decided yes on values the
team's own systems set.
*/
func (m Messenger) Decide(env audience.Env) Surface {
	out := Surface{Launcher: decision(m.Launcher, env), Start: m.Start.Evaluate(Strict(env))}
	if len(m.Tabs) > 0 {
		out.Tabs = map[string]any{}
		for tab, p := range m.Tabs {
			out.Tabs[tab] = decision(p, env)
		}
	}

	return out
}

/*
decision is true, false, or the rule left for the browser.
*/
func decision(p *audience.Program, env audience.Env) any {
	t, residual := p.Partial(env)
	switch t {
	case audience.True:
		return true
	case audience.False:
		return false
	case audience.Unknown:
	}

	return residual
}

/*
asProblems reports whether err carries rule problems.
*/
func asProblems(err error, target **audience.ProblemsError) bool {
	return errors.As(err, target)
}
