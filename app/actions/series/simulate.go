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
|     happen.
|
| The goal and the exit rules are checked before every step, as a
| journey's are. "Try it on me" stays the way to see the real messages.
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
do, and what they would do with its messages: opened, clicked or
replied.
*/
type SimulateInput struct {
	Membership authz.Membership
	ID         ids.UUID
	Version    string
	ContactID  *ids.UUID
	Person     *SimPerson
	Events     []string
	Engaged    []string
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
		s: s, in: in, ws: m.Workspace.ID, variants: in.ID.String(),
		out: &SimTrace{Steps: []SimStep{}, Messages: []SimMessage{}, Actions: []SimAction{}},
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
			return nil
		}
		c, err := q.FindContact(ctx, models.FindContactParams{WorkspaceID: r.ws, ID: *r.in.ContactID})
		if err != nil {
			return notFound(err, "find contact")
		}
		r.person, r.variants = c, c.ID.String()
		return nil
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
		if slices.Contains(r.in.Events, entry.Event) {
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
			WorkspaceID: r.ws, AudienceID: audience, Rules: rules,
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
	if spec.Exit.OnReply && len(r.out.Messages) > 0 && slices.Contains(r.in.Engaged, "replied") {
		return "replied", nil
	}
	if spec.Goal == nil || r.out.Goal {
		return "", nil
	}
	reached := spec.Goal.Event != "" && slices.Contains(r.in.Events, spec.Goal.Event)
	if !reached && len(spec.Goal.Rules) > 0 {
		var err error
		if reached, err = r.matches(ctx, nil, spec.Goal.Rules); err != nil {
			return "", err
		}
	}
	r.out.Goal = reached
	if reached && spec.Exit.OnGoal {
		return "goal", nil
	}
	return "", nil
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
		NodeID: n.ID, Kind: n.Kind, To: to, Subject: m.Subject, Body: m.Body,
	})
	return outcome{path: PathNext, result: "would_send"}
}

/*
waitFor takes the matched path when the person matches, did the event
or engaged as told, and the timed-out path otherwise, at once.
*/
func (r *simRun) waitFor(ctx context.Context, n Node) (outcome, error) {
	s := settingsOf[WaitForSettings](n)
	ok, err := r.test(ctx, s.Rules, s.Event, s.Engagement)
	if err != nil {
		return outcome{}, err
	}
	if ok {
		return outcome{path: PathMatched, result: "matched"}, nil
	}
	return outcome{path: PathTimedOut, result: "timed_out", status: "skipped"}, nil
}

/*
test reports whether the person matches the rules, would do the event
and engaged as told, whichever are given.
*/
func (r *simRun) test(ctx context.Context, rules json.RawMessage, event string, e *Engagement) (bool, error) {
	if len(rules) > 0 {
		ok, err := r.matches(ctx, nil, rules)
		if err != nil || !ok {
			return false, err
		}
	}
	if event != "" && !slices.Contains(r.in.Events, event) {
		return false, nil
	}
	if e == nil {
		return len(rules) > 0 || event != "", nil
	}
	return slices.Contains(r.in.Engaged, e.Action), nil
}

/*
condition takes the first path whose test matches, else everyone else.
*/
func (r *simRun) condition(ctx context.Context, n Node) (outcome, error) {
	for _, p := range settingsOf[ConditionSettings](n).Paths {
		ok, err := r.test(ctx, p.Rules, "", p.Engagement)
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
when it cannot choose.
*/
func (r *simRun) aiPath(ctx context.Context, n Node) outcome {
	settings := settingsOf[AIPathSettings](n)
	if r.s.Picker == nil {
		return outcome{path: PathFallback, result: "ai_fallback"}
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(max(r.s.Config.AITimeoutSeconds, 1))*time.Second)
	defer cancel()
	picked, err := r.s.Picker.Pick(ctx, PickInput{
		WorkspaceID: r.ws, Prompt: settings.Prompt, Paths: settings.Paths,
		Profile: profileOf(&run{people: []models.Contact{r.person}}),
	})
	paths := pathIDs(settings.Paths, func(p AIPath) string { return p.ID })
	if err != nil || !slices.Contains(paths, picked) {
		return outcome{path: PathFallback, result: "ai_fallback"}
	}
	return outcome{path: picked, result: "ai:" + picked}
}

/*
action records a change that would be made, and keeps an attribute it
writes for the steps after it.
*/
func (r *simRun) action(n Node) outcome {
	s := settingsOf[ActionSettings](n)
	detail := map[string]any{}
	switch n.Kind {
	case KindTag, KindUntag:
		detail["tag_ids"] = s.TagIDs
	case KindSetAttribute:
		var value any
		_ = json.Unmarshal(s.Value, &value)
		detail["key"], detail["value"] = s.Key, value
		attrs := map[string]any{}
		_ = json.Unmarshal(r.person.CustomAttributes, &attrs)
		attrs[s.Key] = value
		r.person.CustomAttributes, _ = json.Marshal(attrs)
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
