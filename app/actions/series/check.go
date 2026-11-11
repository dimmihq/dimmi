/*
|--------------------------------------------------------------------------
| Checking A Series
|--------------------------------------------------------------------------
|
| A working copy may be half built; it saves anyway, and the builder
| shows what is still missing next to the step it belongs to. Only a
| series with nothing missing can go live, so a journey never meets a
| step it cannot run: a loop, a path to nowhere, a message without a
| body, a rule that does not parse.
|
*/

package series

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/app/actions/audiences"
	"github.com/dimmihq/dimmi/app/actions/broadcasts"
)

/*
Limits on the size of a series.
*/
const (
	maxNodes    = 200
	maxPaths    = 8
	maxWaitDays = 365
)

/*
Problem is one thing a series misses before it can go live: a code, a
sentence for the builder, and the step and field it is about.
*/
type Problem struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	NodeID  string `json:"node_id,omitempty"`
	Field   string `json:"field,omitempty"`
}

/*
nodeID is the shape of a step or path id.
*/
var nodeID = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,39}$`)

/*
Check lists what the spec misses before it can go live, in the order
the builder shows it.
*/
func Check(spec Spec, subject string) []Problem {
	c := &checker{spec: spec, subject: subject, g: compile(spec)}
	c.nodes()
	c.edges()
	c.rules("entry", "", "entry.rules", spec.Entry.Rules)
	c.rules("exit", "", "exit.rules", spec.Exit.Rules)
	c.goal()

	for _, n := range spec.Nodes {
		c.node(n)
	}

	return c.out
}

/*
checker collects the problems of one spec.
*/
type checker struct {
	spec    Spec
	subject string
	g       graph
	out     []Problem
}

/*
where is the step and field a problem is about.
*/
type where struct {
	node  string
	field string
}

/*
at names the step and field of a problem.
*/
func at(node, field string) where {
	return where{node: node, field: field}
}

/*
add records a problem.
*/
func (c *checker) add(code string, w where, format string, args ...any) {
	c.out = append(c.out, Problem{Code: code, Message: fmt.Sprintf(format, args...), NodeID: w.node, Field: w.field})
}

/*
nodes checks every step has a good, unique id and a known kind, and
that the series starts at its entry.
*/
func (c *checker) nodes() {
	if len(c.spec.Nodes) > maxNodes {
		c.add("too_many_steps", at("", "nodes"), "A series has at most %d steps.", maxNodes)
	}

	seen := map[string]bool{}
	entries := 0

	for _, n := range c.spec.Nodes {
		switch {
		case !nodeID.MatchString(n.ID):
			c.add("step_id_invalid", at(n.ID, "id"), "Step ids use lowercase letters, digits, - and _.")
		case seen[n.ID]:
			c.add("step_id_taken", at(n.ID, "id"), "Two steps share the id %q.", n.ID)
		case !knownKind(n.Kind):
			c.add("step_kind_unknown", at(n.ID, "kind"), "This kind of step does not exist.")
		}

		seen[n.ID] = true
		if n.Kind == KindEntry {
			entries++
		}
	}

	if entries != 1 || c.g.nodes[KindEntry].Kind != KindEntry {
		c.add("entry_missing", at("", "nodes"), "A series starts at exactly one entry step, with the id entry.")
	}
}

/*
edges checks every path leads from a real step by one of its paths to
another real step, and that no path loops back.
*/
func (c *checker) edges() {
	taken := map[string]bool{}

	for _, e := range c.spec.Edges {
		from, ok := c.g.node(e.From)
		if !ok {
			c.add("path_from_unknown", at(e.From, "edges"), "A path starts at a step that does not exist.")
			continue
		}

		if _, ok := c.g.node(e.To); !ok || e.To == KindEntry {
			c.add("path_to_unknown", at(e.From, "edges"), "A path leads to a step that does not exist.")
		}

		path := pathOrNext(e.Path)
		if !slices.Contains(pathsOf(from), path) {
			c.add("path_unknown", at(e.From, "edges"), "This step has no path %q.", path)
		}

		key := e.From + "\x00" + path
		if taken[key] {
			c.add("path_twice", at(e.From, "edges"), "The path %q leads to two steps.", path)
		}

		taken[key] = true
	}

	if c.g.cyclic() {
		c.add("loop", at("", "edges"), "A path leads back to an earlier step. Series cannot loop.")
	}

	c.reach()
}

/*
reach checks every step can be reached and has few enough paths.
*/
func (c *checker) reach() {
	reach := c.g.reachable()
	for _, n := range c.spec.Nodes {
		if !reach[n.ID] {
			c.add("step_unreachable", at(n.ID, ""), "No path leads to this step.")
		}

		if len(c.g.next[n.ID]) > maxPaths {
			c.add("too_many_paths", at(n.ID, ""), "A step has at most %d paths.", maxPaths)
		}
	}
}

/*
rules checks a rule parses as an audience rule.
*/
func (c *checker) rules(code, node, field string, raw json.RawMessage) {
	if len(raw) == 0 || string(raw) == "null" {
		return
	}

	if _, err := audiences.Parse(raw); err != nil {
		c.add(code+"_rules_invalid", at(node, field), "These rules do not read: %s", err.Error())
	}
}

/*
goal checks the goal names an event or rules and a window.
*/
func (c *checker) goal() {
	g := c.spec.Goal
	if g == nil {
		return
	}

	if strings.TrimSpace(g.Event) == "" && len(g.Rules) == 0 {
		c.add("goal_empty", at("", "goal"), "The goal needs an event or rules.")
	}

	if g.Days < 1 || g.Days > 90 {
		c.add("goal_window_invalid", at("", "goal.days"), "The goal window is 1 to 90 days.")
	}

	c.rules("goal", "", "goal.rules", g.Rules)
}

/*
node checks one step's settings.
*/
func (c *checker) node(n Node) {
	switch {
	case IsMessage(n.Kind):
		c.message(n)
	case n.Kind == KindWait:
		c.wait(n)
	case n.Kind == KindWaitFor:
		c.waitFor(n)
	case n.Kind == KindCondition:
		c.condition(n)
	case n.Kind == KindSplit:
		c.split(n)
	case n.Kind == KindAIPath:
		c.aiPath(n)
	default:
		c.action(n)
	}
}

/*
message checks a message has what its channel needs.
*/
func (c *checker) message(n Node) {
	s := settingsOf[MessageSettings](n)
	if strings.TrimSpace(s.Body) == "" {
		c.add("message_body_missing", at(n.ID, "body"), "Write the message.")
	}

	if n.Kind == KindEmail && strings.TrimSpace(s.Subject) == "" {
		c.add("message_subject_missing", at(n.ID, "subject"), "Give the email a subject.")
	}

	if err := broadcasts.CheckVariables(s.Subject + s.Preview + s.Body); err != nil {
		c.add("message_variable_unknown", at(n.ID, "body"),
			"The message uses a detail dimmi doesn't know. Use contact, company, sender or workspace details.")
	}

	if len(s.Recipients) > 0 && c.subject != SubjectCompany {
		c.add("message_recipients_contact", at(n.ID, "recipients"),
			"Only account series pick who in a company gets it.")
	}

	c.rules("message", n.ID, "recipients", s.Recipients)
}

/*
wait checks a wait says how long or until when.
*/
func (c *checker) wait(n Node) {
	s := settingsOf[WaitSettings](n)

	switch s.Mode {
	case "duration":
		c.duration(n.ID, s.Amount, s.Unit)
	case "time":
		if _, _, ok := clockTime(s.At); !ok {
			c.add("wait_time_invalid", at(n.ID, "at"), "Give a time of day like 09:00.")
		}
	case "attribute":
		if strings.TrimSpace(s.Attribute) == "" {
			c.add("wait_attribute_missing", at(n.ID, "attribute"), "Choose the date to wait for.")
		}
	default:
		c.add("wait_mode_invalid", at(n.ID, "mode"), "Wait for a time, until a time of day, or until a date.")
	}
}

/*
duration checks an amount of time a step waits.
*/
func (c *checker) duration(node string, amount int, unit string) {
	d, ok := durationOf(amount, unit)
	if !ok || amount < 1 || d > maxWaitDays*day {
		c.add("wait_duration_invalid", at(node, "amount"), "Wait between a minute and %d days.", maxWaitDays)
	}
}

/*
waitFor checks a wait names what it waits for and for how long.
*/
func (c *checker) waitFor(n Node) {
	s := settingsOf[WaitForSettings](n)
	if len(s.Rules) == 0 && strings.TrimSpace(s.Event) == "" && s.Engagement == nil {
		c.add("wait_for_empty", at(n.ID, ""), "Choose what to wait for: rules, an event, or a reply or click.")
	}

	c.rules("wait_for", n.ID, "rules", s.Rules)
	c.engagement(n.ID, s.Engagement)
	c.duration(n.ID, s.Amount, s.Unit)
}

/*
engagement checks an engagement names an earlier message step.
*/
func (c *checker) engagement(node string, e *Engagement) {
	if e == nil {
		return
	}

	target, ok := c.g.node(e.Node)
	if !ok || !IsMessage(target.Kind) {
		c.add("engagement_step_invalid", at(node, "engagement.node"), "Choose a message step of this series.")
	}

	if !slices.Contains([]string{"opened", "clicked", "replied"}, e.Action) {
		c.add("engagement_action_invalid", at(node, "engagement.action"), "Choose opened, clicked or replied.")
	}
}

/*
knownKind reports whether a kind of step exists.
*/
func knownKind(kind string) bool {
	return slices.Contains([]string{
		KindEntry, KindEmail, KindChat, KindPost, KindSlack, KindWait, KindWaitFor, KindCondition, KindSplit,
		KindAIPath, KindTag, KindUntag, KindSetAttribute, KindTopic, KindWebhook, KindExit,
	}, kind)
}
