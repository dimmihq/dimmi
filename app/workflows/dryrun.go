/*
|--------------------------------------------------------------------------
| Dry Runs
|--------------------------------------------------------------------------
|
| A dry run walks a workflow's definition the way a run would, without
| a run: nothing is written, nothing is sent and nothing leaves dimmi.
| It reads the facts of a real conversation, or of a made-up customer,
| once at the start, and keeps every change the workflow would make in
| memory, so a later branch sees the tag an earlier step would add.
|
| The trigger is checked first, as an event would: its settings, the
| channels, the office hours and the audience. What depends on the
| world at that moment (runs already active, earlier runs, budgets) is
| not checked. Then the steps run one after another:
|
|   - Messages, articles and reply times are said to the driver, which
|     keeps the chat.
|   - Buttons and collect steps ask the driver for the customer's move:
|     a tap, a typed answer, or no answer, which lets the timeout pass.
|   - Waits are skipped, and recorded as skipped.
|   - A step that lets dimmi handle the conversation hands the chat to
|     the driver, which plays the agent's turns and says how it ended.
|   - Actions are recorded as what would run. HTTP requests answer with
|     their mock, or succeed with nothing, and classify steps take the
|     mocked category or ask the classifier.
|
| The driver lives with the caller, which owns the customer, the agent
| and the stream; this file only knows the workflow.
|
*/

package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/ai/actionkit"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/audience"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
maxDrySteps bounds the steps of one dry run.
*/
const maxDrySteps = 200

/*
DryRun is what a dry run walks: the workspace, the definition, the
conversation whose facts it reads or the made-up customer instead, the
trigger's data, the mocks by step id, and the driver that plays the
customer and the agent.
*/
type DryRun struct {
	WorkspaceID  ids.UUID
	Spec         json.RawMessage
	Conversation *ids.UUID
	Person       DryPerson
	Trigger      map[string]any
	Mocks        map[string]actionkit.Mock
	Driver       DryDriver
}

/*
DryPerson is the made-up customer of a dry run with no conversation:
their name and email, the channel they write on, the inbox, and their
custom attributes.
*/
type DryPerson struct {
	Name       string
	Email      string
	Channel    string
	InboxID    string
	Attributes map[string]any
}

/*
DryDriver plays what a dry run does not decide itself.
*/
type DryDriver interface {
	/*
		Said keeps what the workflow said to the customer.
	*/
	Said(ctx context.Context, m DryMessage)
	/*
		Customer returns the customer's answer to what the workflow
		asked.
	*/
	Customer(ctx context.Context, ask DryAsk) (DryAnswer, error)
	/*
		Handle plays the agent's turns of a step that lets dimmi handle
		the conversation, and says how they ended.
	*/
	Handle(ctx context.Context, h DryHandle) (DryHandled, error)
	/*
		Stepped is told of each step once it ran, for whoever watches.
	*/
	Stepped(ctx context.Context, s DryStep)
	/*
		Words returns what the customer wrote so far, oldest first.
	*/
	Words() []string
}

/*
DryMessage is one message the workflow sends: the step, its words and
the buttons it offers.
*/
type DryMessage struct {
	StepID  string   `json:"step_id"`
	Text    string   `json:"text"`
	Choices []string `json:"choices,omitempty"`
}

/*
DryAsk is what the workflow waits on from the customer: the step and
its kind, the question, the buttons, whether only a button will do,
and whether the wait can run out.
*/
type DryAsk struct {
	StepID      string
	Kind        string
	Question    string
	Choices     []string
	ChoicesOnly bool
	CanWait     bool
}

/*
DryAnswer is the customer's move: what they wrote or the button they
tapped, by its number counting from one, or no answer at all.
*/
type DryAnswer struct {
	Text   string
	Choice int
	Silent bool
}

/*
DryHandle is a step that lets dimmi handle the conversation: the step
and the agent it names, if any.
*/
type DryHandle struct {
	StepID  string
	AgentID *ids.UUID
}

/*
DryHandled is how the agent's turns ended: resolved, handed over or
quiet, and why.
*/
type DryHandled struct {
	Outcome string
	Reason  string
}

/*
DryTrace is what a dry run did: whether the workflow would start, every
step and the path it took, the variables set, the actions that would
run, and how it ended.
*/
type DryTrace struct {
	Verdict DryVerdict     `json:"verdict"`
	Steps   []DryStep      `json:"steps"`
	Vars    map[string]any `json:"vars"`
	Actions []DryAction    `json:"actions"`
	End     string         `json:"end"`
}

/*
DryVerdict says whether the trigger would start the workflow, and the
check that refused it when not.
*/
type DryVerdict struct {
	Started    bool   `json:"started"`
	Reason     string `json:"reason,omitempty"`
	FailedRule string `json:"failed_rule,omitempty"`
}

/*
DryStep is one step a dry run took: which, how it went, the path it
left by with the path's label, and what it noted.
*/
type DryStep struct {
	StepID    string         `json:"step_id"`
	Kind      string         `json:"kind"`
	Outcome   string         `json:"outcome"`
	PathID    string         `json:"path_id,omitempty"`
	PathLabel string         `json:"path_label,omitempty"`
	Skipped   bool           `json:"skipped,omitempty"`
	Detail    map[string]any `json:"detail,omitempty"`
}

/*
DryAction is an action the workflow would take: the step, its kind,
what it would do in words, and whether a mock answered for it.
*/
type DryAction struct {
	StepID  string         `json:"step_id"`
	Kind    string         `json:"kind"`
	Summary string         `json:"summary"`
	Detail  map[string]any `json:"detail,omitempty"`
	Mocked  bool           `json:"mocked,omitempty"`
}

/*
The ends of a dry run besides a run's own.
*/
const (
	DryNotStarted = "not_started"
	DryHandedOver = "handed_over"
)

/*
ErrDryRun is returned when a dry run cannot start: the definition does
not read, or the conversation is not the workspace's.
*/
var ErrDryRun = errors.New("workflows: the dry run cannot start")

/*
dry is a dry run while it walks.
*/
type dry struct {
	e      *Engine
	in     DryRun
	spec   workflowspec.Spec
	index  *workflowspec.Index
	subj   *subject
	conv   models.Conversation
	team   string
	out    *DryTrace
	cursor string
}

/*
DryRun walks a definition without a run and returns its trace.
*/
func (e *Engine) DryRun(ctx context.Context, in DryRun) (*DryTrace, error) {
	c, err := compileCandidate(in.Spec)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDryRun, err)
	}
	d := &dry{
		e: e, in: in, spec: c.spec, index: c.index,
		out: &DryTrace{Steps: []DryStep{}, Vars: map[string]any{}, Actions: []DryAction{}},
	}
	if err := d.load(ctx); err != nil {
		return nil, err
	}
	if v, err := d.verdict(ctx, c); err != nil || !v.Started {
		d.out.Verdict, d.out.End = v, DryNotStarted
		return d.out, err
	}
	d.out.Verdict = DryVerdict{Started: true}
	d.cursor = d.index.First()

	return d.out, d.walk(ctx)
}

/*
read runs fn in a short read-only transaction of the workspace.
*/
func (d *dry) read(ctx context.Context, fn func(ctx context.Context, q *models.Queries) error) error {
	return d.e.transaction(ctx, d.in.WorkspaceID, func(ctx context.Context, tx *database.Tx) error {
		return fn(ctx, models.New(tx))
	})
}

/*
load reads the facts conditions read: the real conversation's, or the
made-up customer's.
*/
func (d *dry) load(ctx context.Context) error {
	return d.read(ctx, func(ctx context.Context, q *models.Queries) error {
		if d.in.Conversation != nil {
			conv, err := q.FindConversation(ctx, models.FindConversationParams{
				WorkspaceID: d.in.WorkspaceID, ID: *d.in.Conversation,
			})
			if err != nil {
				return fmt.Errorf("%w: %w", ErrDryRun, err)
			}
			s, err := loadSubject(ctx, q, conv)
			if err != nil {
				return err
			}
			d.subj, d.conv = s, conv
		} else {
			d.conv, d.subj = d.madeUp()
		}
		if err := d.e.withHours(ctx, q, d.subj); err != nil {
			d.subj.hours = nil
		}
		d.subj.trigger = map[string]any{}
		for k, v := range d.in.Trigger {
			d.subj.trigger[k] = v
		}
		d.subj.vars, d.subj.now = d.out.Vars, d.e.Clock.Now()

		return nil
	})
}

/*
madeUp is the conversation and facts of the made-up customer.
*/
func (d *dry) madeUp() (models.Conversation, *subject) {
	p := d.in.Person
	conv := models.Conversation{
		WorkspaceID: d.in.WorkspaceID, Status: "open", AiState: "idle", CreatedAt: d.e.Clock.Now(), IsTest: true,
	}
	if id, err := d.e.IDs.Decode(string(models.PrefixInbox), p.InboxID); err == nil {
		conv.InboxID = id
	}
	contact := models.Contact{WorkspaceID: d.in.WorkspaceID, Kind: "user", CustomAttributes: json.RawMessage("{}")}
	if p.Name != "" {
		contact.Name = &p.Name
	}
	if p.Email != "" {
		contact.Email = &p.Email
	}
	if len(p.Attributes) > 0 {
		contact.CustomAttributes, _ = json.Marshal(p.Attributes)
	}
	channel := p.Channel
	if channel == "" {
		channel = "chat"
	}

	return conv, &subject{conversation: conv, contact: contact, channel: channel}
}

/*
verdict checks the trigger as an event would, leaving out what depends
on the world at that moment. A dry run plays the trigger as the
workflow names it, so its settings are checked only when the trigger's
data names where a change went.
*/
func (d *dry) verdict(ctx context.Context, c candidate) (DryVerdict, error) {
	switch {
	case d.in.Trigger["trigger.to"] != nil && !triggerSettingsMatch(c.spec, d.in.Trigger):
		return DryVerdict{Reason: SkipTrigger}, nil
	case len(c.spec.Channels) > 0 && !slices.Contains(c.spec.Channels, d.subj.channel):
		return DryVerdict{Reason: SkipChannel}, nil
	}
	out := DryVerdict{Started: true}
	err := d.read(ctx, func(ctx context.Context, q *models.Queries) error {
		reason, err := d.e.scheduleReason(ctx, q, &match{conv: d.conv}, c)
		if reason != "" || err != nil {
			out = DryVerdict{Reason: reason}
			return err
		}
		if err := d.subj.withSaved(ctx, q, c.audience); err != nil {
			return err
		}
		if explained := c.audience.Explain(d.subj.env()); explained.Result != audience.True {
			failing, _ := explained.FirstFailing()
			out = DryVerdict{Reason: SkipAudience, FailedRule: failing.Field}
		}
		return nil
	})

	return out, err
}

/*
walk runs the steps until the path ends, the run would end, or the
steps run out.
*/
func (d *dry) walk(ctx context.Context) error {
	for range maxDrySteps {
		if d.cursor == "" {
			d.finish(EndCompleted)
			return nil
		}
		step, ok := d.index.Step(d.cursor)
		if !ok {
			d.finish(EndDefinitionError)
			return nil
		}
		out, err := d.step(ctx, step)
		if err != nil {
			return fmt.Errorf("dry run step %s: %w", step.ID, err)
		}
		d.record(ctx, step, out)
		switch {
		case out.end != "":
			d.finish(out.end)
			return nil
		case out.path != "":
			d.cursor = d.index.Enter(step.ID, out.path)
		default:
			d.cursor = d.index.Next(step.ID)
		}
	}
	d.finish(EndStepBudget)

	return nil
}

/*
finish ends the dry run for a reason, once.
*/
func (d *dry) finish(reason string) {
	if d.out.End == "" {
		d.out.End = reason
	}
}

/*
record keeps a step's trace and tells the driver.
*/
func (d *dry) record(ctx context.Context, step workflowspec.Step, out outcome) {
	s := DryStep{
		StepID: step.ID, Kind: step.Kind, Outcome: out.outcome, PathID: out.path, Detail: out.detail,
		Skipped: out.status == "skipped",
	}
	for _, p := range step.Paths {
		if p.ID == out.path {
			s.PathLabel = p.Label
			if s.PathLabel == "" {
				s.PathLabel = p.Outcome
			}
		}
	}
	d.out.Steps = append(d.out.Steps, s)
	d.in.Driver.Stepped(ctx, s)
}

/*
act records an action the workflow would take.
*/
func (d *dry) act(step workflowspec.Step, summary string, detail map[string]any) outcome {
	d.out.Actions = append(d.out.Actions, DryAction{StepID: step.ID, Kind: step.Kind, Summary: summary, Detail: detail})
	return outcome{outcome: "would_run", acted: true, detail: detail}
}

/*
render fills the variables of a text from the dry run's facts.
*/
func (d *dry) render(ctx context.Context, text string) string {
	if !strings.Contains(text, "{{") {
		return text
	}
	d.refresh()

	return variablePattern().ReplaceAllStringFunc(text, func(found string) string {
		m := variablePattern().FindStringSubmatch(found)
		if v := d.variable(ctx, m[1]); v != "" {
			return v
		}
		return m[2]
	})
}

/*
variable returns one variable's text, empty when it has no value.
*/
func (d *dry) variable(ctx context.Context, name string) string {
	switch name {
	case "contact.first_name":
		full, _ := d.subj.Lookup("contact.name")
		first, _, _ := strings.Cut(fmt.Sprint(orEmpty(full)), " ")
		return first
	case "conversation.number":
		if d.conv.Number == 0 {
			return ""
		}
		return fmt.Sprint(d.conv.Number)
	case "workspace.name":
		name := ""
		_ = d.read(ctx, func(ctx context.Context, q *models.Queries) error {
			ws, err := q.FindWorkspace(ctx, d.in.WorkspaceID)
			name = ws.Name
			return err
		})
		return name
	}
	v, ok := d.subj.Lookup(name)
	if !ok {
		return ""
	}

	return fmt.Sprint(v)
}

/*
refresh hands the facts the conversation as the dry run changed it.
*/
func (d *dry) refresh() {
	d.subj.conversation, d.subj.vars, d.subj.now = d.conv, d.out.Vars, d.e.Clock.Now()
}

/*
decode reads a public id.
*/
func (d *dry) decode(prefix models.Prefix, public string) (ids.UUID, bool) {
	id, err := d.e.IDs.Decode(string(prefix), public)
	return id, err == nil
}
