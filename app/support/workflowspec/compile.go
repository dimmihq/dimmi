/*
|--------------------------------------------------------------------------
| Compiling Definitions
|--------------------------------------------------------------------------
|
| Publishing freezes a definition with what the engine and the list
| need to know without walking the tree again: whether it speaks to the
| customer, and every team, member, tag, schedule and workflow it
| references, so deleting one of them can warn the workflows that rely
| on it. The engine's own index, where each step sits and what follows
| it, is rebuilt in memory from the frozen definition.
|
*/

package workflowspec

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/support/str"

	"github.com/dimmihq/dimmi/app/support/audience"
)

/*
The workflow kinds.
*/
const (
	KindCustomerFacing = "customer_facing"
	KindBackground     = "background"
)

/*
References lists the public ids a definition acts on, by kind.
*/
type References struct {
	Teams       []string `json:"teams,omitempty"`
	Members     []string `json:"members,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Schedules   []string `json:"schedules,omitempty"`
	Workflows   []string `json:"workflows,omitempty"`
	Audiences   []string `json:"audiences,omitempty"`
	Connections []string `json:"connections,omitempty"`
	Agents      []string `json:"agents,omitempty"`
}

/*
Compiled is what a version stores beside its definition.
*/
type Compiled struct {
	Kind                string     `json:"kind"`
	Steps               int        `json:"steps"`
	CustomerFacingSteps []string   `json:"customer_facing_steps"`
	References          References `json:"references"`
}

/*
Compile computes the compiled form of a definition.
*/
func Compile(s Spec) Compiled {
	c := Compiled{Kind: WorkflowKind(s), CustomerFacingSteps: []string{}}
	Walk(s.Steps, func(step Step) {
		c.Steps++
		if speaks(step) {
			c.CustomerFacingSteps = append(c.CustomerFacingSteps, step.ID)
		}

		collectReferences(step, &c.References)
		for _, p := range step.Paths {
			if rule, err := audience.ParseRule(p.Condition); err == nil {
				c.References.Audiences = appendUnique(c.References.Audiences, audience.SavedIDs(rule)...)
			}
		}
	})

	if a, err := audience.Parse(s.Audience); err == nil {
		c.References.Audiences = appendUnique(c.References.Audiences, audience.SavedIDs(a.Rules)...)
	}

	if s.Schedule.ScheduleID != "" {
		c.References.Schedules = appendUnique(c.References.Schedules, s.Schedule.ScheduleID)
	}

	return c
}

/*
WorkflowKind says whether a definition is customer-facing: its trigger
starts conversations, or any of its steps speaks to the customer.
*/
func WorkflowKind(s Spec) string {
	if t, ok := FindTrigger(s.Trigger); ok && t.StartsConversation() {
		return KindCustomerFacing
	}

	facing := false
	Walk(s.Steps, func(step Step) { facing = facing || speaks(step) })

	if facing {
		return KindCustomerFacing
	}

	return KindBackground
}

/*
speaks reports whether a step speaks to the customer.
*/
func speaks(s Step) bool {
	if k, ok := FindKind(s.Kind); ok && k.CustomerFacing {
		return true
	}

	if s.Kind != KindClose {
		return false
	}

	var cl CloseSettings
	_ = json.Unmarshal(s.Settings, &cl)

	return cl.AskRating
}

/*
Walk calls fn for every step, depth first, in reading order.
*/
func Walk(steps []Step, fn func(Step)) {
	for _, s := range steps {
		fn(s)
		for _, p := range s.Paths {
			Walk(p.Steps, fn)
		}
	}
}

/*
collectReferences records the ids a step's settings name.
*/
func collectReferences(s Step, r *References) {
	var refs struct {
		MemberID         string     `json:"member_id"`
		TeamID           string     `json:"team_id"`
		TagIDs           []string   `json:"tag_ids"`
		ScheduleID       string     `json:"schedule_id"`
		WorkflowID       string     `json:"workflow_id"`
		ConnectionID     string     `json:"connection_id"`
		AgentID          string     `json:"agent_id"`
		MentionMemberIDs []string   `json:"mention_member_ids"`
		Until            *WaitUntil `json:"until"`
	}

	if json.Unmarshal(s.Settings, &refs) != nil {
		return
	}

	r.Members = appendUnique(r.Members, append([]string{refs.MemberID}, refs.MentionMemberIDs...)...)
	r.Teams = appendUnique(r.Teams, refs.TeamID)
	r.Tags = appendUnique(r.Tags, refs.TagIDs...)
	r.Schedules = appendUnique(r.Schedules, refs.ScheduleID)
	r.Workflows = appendUnique(r.Workflows, refs.WorkflowID)
	r.Connections = appendUnique(r.Connections, refs.ConnectionID)
	r.Agents = appendUnique(r.Agents, refs.AgentID)

	if refs.Until != nil {
		r.Schedules = appendUnique(r.Schedules, refs.Until.ScheduleID)
	}
}

/*
appendUnique appends the non-empty values not already present.
*/
func appendUnique(list []string, values ...string) []string {
	for _, v := range values {
		if v != "" && !slices.Contains(list, v) {
			list = append(list, v)
		}
	}

	return list
}

/*
AssignIDs gives every step and path without an id a fresh one, so a
definition written by hand or by an agent can leave them out.
*/
func AssignIDs(s *Spec) {
	s.Steps = assignSteps(s.Steps)
}

/*
assignSteps assigns ids within a list of steps.
*/
func assignSteps(steps []Step) []Step {
	out := make([]Step, len(steps))
	for i, s := range steps {
		if s.ID == "" {
			s.ID = NewID("s_")
		}

		paths := make([]Path, len(s.Paths))
		for j, p := range s.Paths {
			if p.ID == "" {
				p.ID = NewID("p_")
			}

			p.Steps = assignSteps(p.Steps)
			if p.Steps == nil {
				p.Steps = []Step{}
			}

			paths[j] = p
		}

		if s.Paths != nil {
			s.Paths = paths
		}

		s.Paths = withOpenPaths(s)
		out[i] = s
	}

	return out
}

/*
openPaths are the fixed paths a step of a kind always shows, so steps can
follow each outcome: every outcome of an agent step, and the outcomes
every step of its kind must have.
*/
func openPaths(k Kind) []string {
	if k.Key == KindDimmi {
		return k.Fixed
	}

	return k.Required
}

/*
withOpenPaths adds the open paths a step leaves out, empty, so a step an
agent or a script wrote reads in the builder like one made there.
*/
func withOpenPaths(s Step) []Path {
	k, ok := FindKind(s.Kind)
	if !ok {
		return s.Paths
	}

	paths := s.Paths
	for _, outcome := range openPaths(k) {
		if !slices.ContainsFunc(paths, func(p Path) bool { return p.Outcome == outcome }) {
			paths = append(paths, Path{ID: NewID("p_"), Outcome: outcome, Steps: []Step{}})
		}
	}

	return paths
}

/*
NewID returns a fresh step or path id with the prefix.
*/
func NewID(prefix string) string {
	return prefix + strings.ToLower(str.Random(10))
}
