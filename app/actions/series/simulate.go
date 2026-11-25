/*
|--------------------------------------------------------------------------
| Dry Simulations
|--------------------------------------------------------------------------
|
| A teammate walks one person through a series before anyone travels it:
| a real contact, or a made-up person, through the draft or the live
| version. Nothing is sent, written or posted, and no wait holds them:
|
|   - Every wait is skipped, and recorded as skipped.
|   - Message steps say what they would send, and to whom.
|   - Checks read the person as they are, with the changes earlier
|     steps would make kept in memory. Engagement with a message, and
|     events the person would do, are what the teammate says they do.
|   - A split takes the winner, or a variant picked the way a journey's
|     is; an AI path asks the same picker a journey does.
|   - Tags, attributes, topics and webhooks are recorded as what would
|     happen. Tags and attributes are kept in memory, so later rules
|     read them.
|   - An extract step takes its mock or asks the AI a journey asks, and
|     a run action step answers with its mock and never runs the
|     action; the values both find are kept as attributes.
|
| The goal and the exit rules are checked before every step, as a
| journey's are. With a scenario in words, the decision model says what
| the person does instead; see simulate_scenario.go.
|
*/

package series

import (
	"context"
	"encoding/json"
	"hash/fnv"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/audiences"
	"github.com/dimmihq/dimmi/app/actions/broadcasts"
	"github.com/dimmihq/dimmi/app/ai/actionkit"
	"github.com/dimmihq/dimmi/app/ai/aidecision"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxSimSteps bounds the steps of one dry simulation.
*/
const maxSimSteps = 200

/*
The failures of dry simulations.
*/
var (
	ErrSimNotLive = fault.New(fault.Invalid, "series_not_live", "This series has no live version.").
			OnField("version")
	ErrSimNoPerson = fault.New(fault.Invalid, "series_simulation_invalid",
		"Pick a contact or describe a made-up person.").OnField("contact_id")
)

/*
SimulateInput is a dry simulation: the series and which copy, the real
contact or the made-up person who travels it, the events they would
do, what they would do with its messages (opened, clicked or replied),
or a scenario that says how they behave in words, and the mocks of
its extract and run action steps, by step id or by action id.
*/
type SimulateInput struct {
	Membership authz.Membership
	ID         ids.UUID
	Version    string
	ContactID  *ids.UUID
	Person     *SimPerson
	Events     []string
	Engaged    []string
	Scenario   string
	Mocks      map[string]actionkit.Mock
}

/*
SimPerson is the made-up person of a dry simulation.
*/
type SimPerson struct {
	Name       string         `json:"name,omitempty"`
	Email      string         `json:"email,omitempty"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

/*
simRun is one dry simulation while it walks.
*/
type simRun struct {
	s        *Series
	in       SimulateInput
	ws       ids.UUID
	g        graph
	person   models.Contact
	out      *SimTrace
	variants string
	tags     map[string]bool
	vars     map[string]string
	decided  map[string]bool
}

/*
Simulate walks one person through a series without sending anything.
*/
func (s *Series) Simulate(ctx context.Context, in SimulateInput) (*SimTrace, error) {
	m := in.Membership
	if !m.Can("series:write") {
		return nil, exceptions.ErrForbidden
	}
	if in.ContactID == nil && in.Person == nil {
		return nil, ErrSimNoPerson
	}
	r := &simRun{
		s: s, in: in, ws: m.Workspace.ID, variants: in.ID.String(), tags: map[string]bool{},
		decided: map[string]bool{},
		out:     &SimTrace{Steps: []SimStep{}, Messages: []SimMessage{}, Actions: []SimAction{}},
	}
	if err := r.load(ctx); err != nil {
		return nil, err
	}
	entered, err := r.enters(ctx)
	if err != nil || !entered {
		r.out.End = "not_entered"
		return r.out, err
	}
	r.out.Entered = true

	return r.out, r.walk(ctx)
}

/*
load reads the copy of the series and the person.
*/
func (r *simRun) load(ctx context.Context) error {
	return r.s.inWorkspace(ctx, r.ws, func(ctx context.Context, q *models.Queries) error {
		raw, err := r.copyOf(ctx, q)
		if err != nil {
			return err
		}
		spec, err := decode(raw)
		if err != nil {
			return err
		}
		r.g = compile(spec)
		if r.in.ContactID == nil {
			r.person = madeUpPerson(r.ws, *r.in.Person)
		} else {
			c, err := q.FindContact(ctx, models.FindContactParams{WorkspaceID: r.ws, ID: *r.in.ContactID})
			if err != nil {
				return notFound(err, "find contact")
			}
			r.person, r.variants = c, c.ID.String()
		}
		row, err := find(ctx, q, r.ws, r.in.ID)
		if err != nil {
			return err
		}
		r.vars, err = broadcasts.PreviewVariables(ctx, q, row.SenderMemberID, r.person)
		return err
	})
}

/*
copyOf reads the definition of the series' draft or its live version.
*/
func (r *simRun) copyOf(ctx context.Context, q *models.Queries) (json.RawMessage, error) {
	row, err := find(ctx, q, r.ws, r.in.ID)
	if err != nil || r.in.Version != "live" {
		return row.Working, err
	}
	if row.LiveVersionID == nil {
		return nil, ErrSimNotLive
	}
	v, err := q.FindSeriesVersion(ctx, models.FindSeriesVersionParams{WorkspaceID: r.ws, ID: *row.LiveVersionID})
	if err != nil {
		return nil, notFound(err, "find series version")
	}
	return v.Spec, nil
}

/*
madeUpPerson is the contact a made-up person stands for.
*/
func madeUpPerson(ws ids.UUID, p SimPerson) models.Contact {
	c := models.Contact{WorkspaceID: ws, Kind: "user", CustomAttributes: json.RawMessage("{}")}
	if p.Name != "" {
		c.Name = &p.Name
	}
	if p.Email != "" {
		c.Email = &p.Email
	}
	if len(p.Attributes) > 0 {
		c.CustomAttributes, _ = json.Marshal(p.Attributes)
	}
	return c
}

/*
enters reports whether the person would enter: they match the entry's
audience or rules, or would do its event.
*/
func (r *simRun) enters(ctx context.Context) (bool, error) {
	entry := r.g.spec.Entry
	if entry.Event != "" {
		if r.in.Scenario != "" || slices.Contains(r.in.Events, entry.Event) {
			return true, nil
		}
		r.out.Reason = "event_not_done"
		return false, nil
	}
	ok, err := r.matches(ctx, r.s.audienceID(entry.AudienceID), entry.Rules)
	if err == nil && !ok {
		r.out.Reason = "entry_rules"
	}
	return ok, err
}

/*
matches reports whether the person matches the rules or audience.
*/
func (r *simRun) matches(ctx context.Context, audience *ids.UUID, rules json.RawMessage) (bool, error) {
	var got []ids.UUID
	err := r.s.inWorkspace(ctx, r.ws, func(ctx context.Context, q *models.Queries) error {
		var err error
		got, err = r.s.Audiences.Matching(ctx, q, audiences.MatchInput{
			WorkspaceID: r.ws, AudienceID: audience, Rules: rules, Adjust: r.adjust,
		}, []models.Contact{r.person})
		return err
	})
	return len(got) > 0, err
}

/*
walk takes the person from the entry until a path ends.
*/
func (r *simRun) walk(ctx context.Context) error {
	at := KindEntry
	for range maxSimSteps {
		if end, err := r.guard(ctx); end != "" || err != nil {
			r.out.End = end
			return err
		}
		n, ok := r.g.node(at)
		if !ok {
			r.out.End = "node_removed"
			return nil
		}
		out, err := r.enter(ctx, n)
		if err != nil {
			return err
		}
		r.out.Steps = append(r.out.Steps, SimStep{
			NodeID: n.ID, Kind: n.Kind, Path: out.path, Result: out.result, Skipped: out.status == "skipped",
			Detail: out.details,
		})
		if out.end != "" {
			r.out.End = out.end
			return nil
		}
		to, ok := r.g.target(n.ID, out.path)
		if !ok {
			r.out.End = "finished"
			return nil
		}
		at = to
	}
	r.out.End = "step_budget"
	return nil
}

/*
guard checks the exit rules, a reply that exits, and the goal before a
step; it returns why the person would leave, empty when they stay.
*/
func (r *simRun) guard(ctx context.Context) (string, error) {
	spec := r.g.spec
	if len(spec.Exit.Rules) > 0 {
		ok, err := r.matches(ctx, nil, spec.Exit.Rules)
		if err != nil || ok {
			return map[bool]string{true: "exit_rules"}[ok], err
		}
	}
	if spec.Exit.OnReply && len(r.out.Messages) > 0 {
		replied, err := r.engaged(ctx, &Engagement{Action: "replied"}, asking{quiet: true})
		if err != nil || replied {
			return map[bool]string{true: "replied"}[replied], err
		}
	}
	if spec.Goal == nil || r.out.Goal {
		return "", nil
	}
	reached, err := r.goalReached(ctx, spec.Goal)
	if err != nil {
		return "", err
	}
	r.out.Goal = reached
	if reached && spec.Exit.OnGoal {
		return "goal", nil
	}
	return "", nil
}

/*
goalReached reports whether the person does the goal's event or
matches its rules.
*/
func (r *simRun) goalReached(ctx context.Context, goal *Goal) (bool, error) {
	if goal.Event != "" {
		did, err := r.did(ctx, goal.Event, asking{quiet: true})
		if err != nil || did {
			return did, err
		}
	}
	if len(goal.Rules) == 0 {
		return false, nil
	}
	return r.matches(ctx, nil, goal.Rules)
}

/*
enter runs one step for the dry simulation.
*/
func (r *simRun) enter(ctx context.Context, n Node) (outcome, error) {
	switch {
	case n.Kind == KindEntry:
		return outcome{path: PathNext, result: "entered"}, nil
	case n.Kind == KindExit:
		return outcome{end: "finished", result: "exit_step"}, nil
	case IsMessage(n.Kind):
		return r.message(n), nil
	case n.Kind == KindWait:
		return outcome{path: PathNext, result: "waited", status: "skipped"}, nil
	case n.Kind == KindWaitFor:
		return r.waitFor(ctx, n)
	case n.Kind == KindCondition:
		return r.condition(ctx, n)
	case n.Kind == KindSplit:
		return r.split(n), nil
	case n.Kind == KindAIPath:
		return r.aiPath(ctx, n), nil
	case n.Kind == KindExtract:
		return r.extract(ctx, n), nil
	case n.Kind == KindRunAction:
		return r.runAction(n), nil
	case n.Kind == KindWebhook:
		url := settingsOf[WebhookSettings](n).URL
		r.act(n, map[string]any{"url": url})
		return outcome{path: PathNext, result: "webhook:not_sent", details: map[string]any{"url": url}}, nil
	}

	return r.action(n), nil
}

/*
message records what a message step would send, or the path it would
take when the person cannot get it.
*/
func (r *simRun) message(n Node) outcome {
	m := settingsOf[MessageSettings](n)
	to := ""
	if r.person.Email != nil {
		to = *r.person.Email
	}
	if n.Kind == KindEmail && to == "" {
		details := map[string]any{"reasons": "no_email"}
		if r.g.hasPath(n.ID, PathFailed) {
			return outcome{path: PathFailed, result: "not_sent", details: details}
		}
		return outcome{end: "undeliverable", result: "not_sent", details: details}
	}
	r.out.Messages = append(r.out.Messages, SimMessage{
		NodeID: n.ID, Kind: n.Kind, To: to,
		Subject: broadcasts.FillVariables(m.Subject, r.vars), Body: broadcasts.FillVariables(m.Body, r.vars),
	})
	return outcome{path: PathNext, result: "would_send"}
}

/*
waitFor takes the matched path when the person matches, did the event
or engaged as told, and the timed-out path otherwise, at once.
*/
func (r *simRun) waitFor(ctx context.Context, n Node) (outcome, error) {
	s := settingsOf[WaitForSettings](n)
	ok, err := r.test(ctx, check{rules: s.Rules, event: s.Event, engagement: s.Engagement})
	if err != nil {
		return outcome{}, err
	}
	if ok {
		return outcome{path: PathMatched, result: "matched"}, nil
	}
	return outcome{path: PathTimedOut, result: "timed_out", status: "skipped"}, nil
}

/*
condition takes the first path whose test matches, else everyone else.
*/
func (r *simRun) condition(ctx context.Context, n Node) (outcome, error) {
	for _, p := range settingsOf[ConditionSettings](n).Paths {
		ok, err := r.test(ctx, check{rules: p.Rules, engagement: p.Engagement, label: p.Label})
		if err != nil {
			return outcome{}, err
		}
		if ok {
			return outcome{path: p.ID, result: "matched", details: map[string]any{"label": p.Label}}, nil
		}
	}
	return outcome{path: PathElse, result: "else"}, nil
}

/*
split takes the winner once there is one, else the variant picked by
weight from a fixed hash of the person, as a journey's would be.
*/
func (r *simRun) split(n Node) outcome {
	s := settingsOf[SplitSettings](n)
	if s.Winner != "" {
		return outcome{path: s.Winner, result: "winner"}
	}
	total := 0
	for _, v := range s.Variants {
		total += max(v.Weight, 0)
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(r.variants + ":" + n.ID))
	pick := int(h.Sum32() % uint32(max(total, 1)))
	for _, v := range s.Variants {
		if pick < v.Weight {
			return outcome{path: v.ID, result: "variant", details: map[string]any{"label": v.Label}}
		}
		pick -= max(v.Weight, 0)
	}
	return outcome{result: "variant"}
}

/*
aiPath asks the picker a journey asks, and takes the fallback path
when it cannot choose. With a scenario, the picker reads it too.
*/
func (r *simRun) aiPath(ctx context.Context, n Node) outcome {
	settings := settingsOf[AIPathSettings](n)
	if r.s.Decisions == nil {
		return outcome{path: PathFallback, result: "ai_fallback"}
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(max(r.s.Config.AITimeoutSeconds, 1))*time.Second)
	defer cancel()
	options := make([]aidecision.Option, 0, len(settings.Paths))
	for _, p := range settings.Paths {
		options = append(options, aidecision.Option{ID: p.ID, Label: p.Label, Description: p.Description})
	}
	profile := profileOf(&run{people: []models.Contact{r.person}})
	if r.in.Scenario != "" {
		profile = r.scenarioContext()
	}
	chosen, err := r.s.Decisions.Choose(ctx, aidecision.Choice{
		Subject:      aidecision.Subject{WorkspaceID: r.ws, Context: profile},
		Instructions: settings.Prompt, Options: options,
	})
	picked := chosen.OptionID
	paths := pathIDs(settings.Paths, func(p AIPath) string { return p.ID })
	if err != nil || !slices.Contains(paths, picked) {
		return outcome{path: PathFallback, result: "ai_fallback"}
	}
	return outcome{path: picked, result: "ai:" + picked}
}

/*
action records a change that would be made, and keeps a tag or an
attribute it writes for the steps after it.
*/
func (r *simRun) action(n Node) outcome {
	s := settingsOf[ActionSettings](n)
	detail := map[string]any{}
	switch n.Kind {
	case KindTag, KindUntag:
		detail["tag_ids"] = s.TagIDs
		for _, t := range s.TagIDs {
			r.tags[t] = n.Kind == KindTag
		}
	case KindSetAttribute:
		var value any
		_ = json.Unmarshal(s.Value, &value)
		detail["key"], detail["value"] = s.Key, value
		r.setAttribute(s.Key, value)
	case KindTopic:
		detail["topic_id"], detail["subscribed"] = s.TopicID, s.Subscribed
	}
	r.act(n, detail)
	return outcome{path: PathNext, result: n.Kind}
}

/*
act records a change.
*/
func (r *simRun) act(n Node, detail map[string]any) {
	r.out.Actions = append(r.out.Actions, SimAction{NodeID: n.ID, Kind: n.Kind, Detail: detail})
}
