/*
|--------------------------------------------------------------------------
| Matching Workflows
|--------------------------------------------------------------------------
|
| An event offers a conversation to the live workflows of its trigger,
| in list order. Each one is checked in a fixed order and the first
| check that fails is its verdict, written down so "why didn't this
| run?" always has an answer. A customer-facing workflow that starts
| holds the conversation, so only the first one starts; background
| workflows all start, and run one after another in list order so the
| lower one always has the last word. A workflow never starts from its
| own changes, and a chain of workflows triggering each other stops at
| three.
|
*/

package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/audience"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
maxChain is the longest chain of workflows triggering each other.
*/
const maxChain = 3

/*
The skip reasons of a verdict.
*/
const (
	SkipTrigger            = "trigger_settings"
	SkipChannel            = "channel"
	SkipAudience           = "audience"
	SkipLoopGuard          = "loop_guard"
	SkipChainDepth         = "chain_depth"
	SkipAlreadyRunning     = "already_running"
	SkipReentry            = "reentry"
	SkipConversationBudget = "conversation_budget"
	SkipHeld               = "held"
	SkipLowerPriority      = "lower_priority"
	SkipExclusive          = "exclusive"
)

/*
Event is something that may start workflows on a conversation: the
trigger, the id of the event for idempotency, the chain of workflows
that caused it, and the trigger's own data, such as the message. Only,
when set, offers the event to that one workflow, as a timer armed for
it does.
*/
type Event struct {
	WorkspaceID    ids.UUID       `json:"workspace_id"`
	ConversationID ids.UUID       `json:"conversation_id"`
	Trigger        string         `json:"trigger"`
	EventID        string         `json:"event_id"`
	Chain          []ids.UUID     `json:"chain,omitempty"`
	Data           map[string]any `json:"data,omitempty"`
	Only           *ids.UUID      `json:"only,omitempty"`
}

/*
Verdict is one workflow's answer to an event.
*/
type Verdict struct {
	WorkflowID string `json:"workflow_id"`
	VersionID  string `json:"version_id,omitempty"`
	Verdict    string `json:"verdict"`
	Reason     string `json:"reason,omitempty"`
	FailedRule string `json:"failed_rule,omitempty"`
}

/*
match is the state of matching one event.
*/
type match struct {
	ev       Event
	conv     models.Conversation
	subject  *subject
	active   []models.WorkflowRun
	claimed  bool
	facing   bool
	cut      bool
	started  []ids.UUID
	verdicts []Verdict
	budget   *int32
}

/*
MatchInTx offers the conversation to the event's workflows inside the
caller's transaction and starts the runs that pass. The runs advance
from a job once the transaction commits.
*/
func (e *Engine) MatchInTx(ctx context.Context, tx *database.Tx, ev Event) error {
	q := models.New(tx)
	found, err := e.candidates(ctx, q, ev.WorkspaceID, ev.Trigger)
	if err != nil {
		return err
	}

	found = onlyFor(found, ev.Only)
	if len(found) == 0 {
		return nil
	}

	conv, err := q.FindConversation(ctx, models.FindConversationParams{
		WorkspaceID: ev.WorkspaceID, ID: ev.ConversationID,
	})
	if err != nil {
		return fmt.Errorf("find conversation: %w", err)
	}

	if !matchable(conv) {
		return nil
	}

	m, err := e.prepare(ctx, q, ev, conv)
	if err != nil {
		return err
	}

	for _, c := range found {
		if err := e.consider(ctx, q, m, c); err != nil {
			return err
		}
	}

	return e.record(ctx, tx, m)
}

/*
Match runs MatchInTx in a transaction of its own, for events matched
from a job.
*/
func (e *Engine) Match(ctx context.Context, ev Event) error {
	return e.transaction(ctx, ev.WorkspaceID, func(ctx context.Context, tx *database.Tx) error {
		return e.MatchInTx(ctx, tx, ev)
	})
}

/*
matchable reports whether workflows may run on the conversation at all.
*/
func matchable(c models.Conversation) bool {
	return !c.Spam && c.DeletedAt == nil
}

/*
prepare loads what every candidate's checks read.
*/
func (e *Engine) prepare(ctx context.Context, q *models.Queries, ev Event, conv models.Conversation) (*match, error) {
	s, err := loadSubject(ctx, q, conv)
	if err != nil {
		return nil, err
	}

	if err := e.withHours(ctx, q, s); err != nil {
		return nil, err
	}

	s.now, s.trigger = e.Clock.Now(), triggerFacts(ctx, q, ev)
	if contactBlocked(s) {
		return nil, nil
	}

	active, err := q.ActiveWorkflowRuns(ctx, models.ActiveWorkflowRunsParams{
		WorkspaceID: conv.WorkspaceID, ConversationID: &conv.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("list active runs: %w", err)
	}

	return &match{ev: ev, conv: conv, subject: s, active: active, claimed: conv.WorkflowRunID != nil}, nil
}

/*
triggerFacts is the trigger's data for conditions, with the text of the
message that triggered it.
*/
func triggerFacts(ctx context.Context, q *models.Queries, ev Event) map[string]any {
	facts := map[string]any{}
	maps.Copy(facts, ev.Data)

	raw, _ := ev.Data["message_id"].(string)
	id, err := ids.Parse(raw)
	if err != nil {
		return facts
	}

	msg, err := q.FindMessage(ctx, models.FindMessageParams{
		WorkspaceID: ev.WorkspaceID, ConversationID: ev.ConversationID, ID: id,
	})
	if err == nil && msg.Body != nil {
		facts["message.body"] = *msg.Body
	}

	return facts
}

/*
contactBlocked reports whether the contact was blocked or erased.
*/
func contactBlocked(s *subject) bool {
	return s.contact.BlockedAt != nil || s.contact.DeletedAt != nil
}

/*
consider checks one candidate and starts its run when it passes.
*/
func (e *Engine) consider(ctx context.Context, q *models.Queries, m *match, c candidate) error {
	if m == nil {
		return nil
	}

	reason, rule, err := e.check(ctx, q, m, c)
	if err != nil {
		return err
	}

	v := Verdict{
		WorkflowID: models.PublicID(models.PrefixWorkflow, c.workflowID), Verdict: "skipped",
		Reason: reason, FailedRule: rule,
	}
	if c.versionID != nil {
		v.VersionID = models.PublicID(models.PrefixWorkflowVersion, *c.versionID)
	}

	if reason == "" {
		started, err := e.start(ctx, q, m, c)
		if err != nil {
			return err
		}

		v.Verdict = "started"
		if !started {
			v.Verdict, v.Reason = "skipped", SkipAlreadyRunning
		}
	}

	m.verdicts = append(m.verdicts, v)

	return nil
}

/*
check returns the first reason the candidate may not start, empty when
it may, and the condition that failed when the audience refused.
*/
func (e *Engine) check(ctx context.Context, q *models.Queries, m *match, c candidate) (string, string, error) {
	facing := c.kind == workflowspec.KindCustomerFacing
	switch {
	case m.cut:
		return SkipExclusive, "", nil
	case !triggerSettingsMatch(c.spec, m.ev.Data):
		return SkipTrigger, "", nil
	case len(c.spec.Channels) > 0 && !slices.Contains(c.spec.Channels, m.subject.channel):
		return SkipChannel, "", nil
	}

	if reason, err := e.scheduleReason(ctx, q, m, c); reason != "" || err != nil {
		return reason, "", err
	}

	if err := m.subject.withSaved(ctx, q, c.audience); err != nil {
		return "", "", err
	}

	if explained := c.audience.Explain(m.subject.env()); explained.Result != audience.True {
		failing, _ := explained.FirstFailing()
		return SkipAudience, failing.Field, nil
	}

	if slices.Contains(m.ev.Chain, c.workflowID) {
		return SkipLoopGuard, "", nil
	}

	if len(m.ev.Chain) >= maxChain {
		return SkipChainDepth, "", nil
	}

	if slices.ContainsFunc(m.active, func(r models.WorkflowRun) bool { return r.WorkflowID == c.workflowID }) {
		return SkipAlreadyRunning, "", nil
	}

	if reason, err := e.reentry(ctx, q, m, c); reason != "" || err != nil {
		return reason, "", err
	}

	if reason, err := e.budget(ctx, q, m); reason != "" || err != nil {
		return reason, "", err
	}

	return holdReason(m, facing), "", nil
}

/*
triggerSettingsMatch applies the trigger's own settings: a status or
assignment change only counts when it went to one of the targets the
workflow names.
*/
func triggerSettingsMatch(spec workflowspec.Spec, data map[string]any) bool {
	var settings struct {
		To []string `json:"to"`
	}

	if json.Unmarshal(spec.TriggerSettings, &settings) != nil || len(settings.To) == 0 {
		return true
	}

	to, _ := data["trigger.to"].(string)

	return slices.Contains(settings.To, to) || slices.Contains(settings.To, "any")
}

/*
holdReason refuses a customer-facing workflow when the conversation is
already held, or an earlier workflow took it for this event.
*/
func holdReason(m *match, facing bool) string {
	switch {
	case facing && m.facing:
		return SkipLowerPriority
	case facing && m.claimed:
		return SkipHeld
	}

	return ""
}

/*
reentry applies the workflow's rule on running again. A customer
waiting for the team fires once per waiting period by its own timer,
so its rule is never counted across periods.
*/
func (e *Engine) reentry(ctx context.Context, q *models.Queries, m *match, c candidate) (string, error) {
	mode := c.spec.Reentry.Mode
	if mode == "" || mode == workflowspec.ReentryEveryTime || c.spec.Trigger == workflowspec.TriggerTeammateQuiet {
		return "", nil
	}

	last, err := q.WorkflowRanOnConversation(ctx, models.WorkflowRanOnConversationParams{
		WorkspaceID: m.conv.WorkspaceID, WorkflowID: c.workflowID, ConversationID: &m.conv.ID,
	})
	if err != nil {
		return "", fmt.Errorf("check earlier runs: %w", err)
	}

	if last.IsZero() {
		return "", nil
	}

	if mode == workflowspec.ReentryOnce {
		return SkipReentry, nil
	}

	cooldown, _ := workflowspec.ParseDuration(c.spec.Reentry.Cooldown)
	if e.Clock.Now().Sub(last) < cooldown {
		return SkipReentry, nil
	}

	return "", nil
}

/*
budget refuses new runs on a conversation whose runs already executed
their hourly share of steps.
*/
func (e *Engine) budget(ctx context.Context, q *models.Queries, m *match) (string, error) {
	if m.budget == nil {
		steps, err := q.CountConversationStepsSince(ctx, models.CountConversationStepsSinceParams{
			WorkspaceID: m.conv.WorkspaceID, ConversationID: &m.conv.ID, Since: e.Clock.Now().Add(-time.Hour),
		})
		if err != nil {
			return "", fmt.Errorf("count conversation steps: %w", err)
		}

		m.budget = &steps
	}

	if int(*m.budget) >= e.Limits.StepsPerConversationHour {
		return SkipConversationBudget, nil
	}

	return "", nil
}

/*
record writes the match log and queues the started runs, in list order.
*/
func (e *Engine) record(ctx context.Context, tx *database.Tx, m *match) error {
	if m == nil || len(m.verdicts) == 0 {
		return nil
	}

	verdicts, err := json.Marshal(m.verdicts)
	if err != nil {
		return fmt.Errorf("encode verdicts: %w", err)
	}

	now := e.Clock.Now()
	if err := models.New(tx).CreateWorkflowMatchLog(ctx, models.CreateWorkflowMatchLogParams{
		ID: ids.NewV7At(now), WorkspaceID: m.conv.WorkspaceID, ConversationID: m.conv.ID, Trigger: m.ev.Trigger,
		EventID: &m.ev.EventID, Verdicts: verdicts, Now: now,
	}); err != nil {
		return fmt.Errorf("record match log: %w", err)
	}

	if len(m.started) == 0 {
		return nil
	}

	return e.queueBatch(ctx, tx, m.conv.WorkspaceID, m.started)
}

/*
onlyFor keeps the one workflow an event is for, or every candidate when
it is for any.
*/
func onlyFor(found []candidate, only *ids.UUID) []candidate {
	if only == nil {
		return found
	}

	var out []candidate
	for _, c := range found {
		if c.workflowID == *only {
			out = append(out, c)
		}
	}

	return out
}
