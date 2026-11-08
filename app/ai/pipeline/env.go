/*
|--------------------------------------------------------------------------
| Run Environment
|--------------------------------------------------------------------------
|
| What a run loads once, inside its first short transaction, and every
| stage reads afterwards: the workspace, its agent, the conversation and
| its customer, the trigger message, the newest public turns, the
| connection each model role resolves to, and the facts the gate
| decides on.
|
| The transcript is the newest turns, never the oldest: the customer's
| messages since the last reply are what the agent answers, and the
| turns before them are context.
|
*/

package pipeline

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/officehours"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
transcriptSize is how many public turns a run loads.
*/
const transcriptSize = 30

/*
env is the run's loaded world.
*/
type env struct {
	Workspace     models.Workspace
	Agent         models.AiAgent
	Conversation  models.Conversation
	Contact       *models.Contact
	Trigger       models.Message
	History       []models.Message
	Pending       []models.Message
	Newest        ids.UUID
	ReplyAfter    string
	ChannelType   string
	AnswerKey     *models.AiProviderKey
	ClassifierKey *models.AiProviderKey
	Facts         facts
	Triage        triageFacts
	Version       string
	Defaults      map[string]string
	WorkflowStep  *workflowspec.AgentHandling
	Procedures    []liveProcedure
	now           time.Time
}

/*
facts are what the gate reads besides the rows themselves.
*/
type facts struct {
	spendCapped    bool
	assigneeActive bool
	onlineMembers  int64
	withinHours    bool
	hoursErr       error
}

/*
errTriggerMissing marks a trigger message that no longer exists.
*/
var errTriggerMissing = errors.New("trigger message missing")

/*
loadEnv reads everything the gate and stages need. A missing agent
returns a nil env.
*/
func (r *Runner) loadEnv(ctx context.Context, tx *database.Tx, args Args) (*env, error) {
	q := models.New(tx)
	conv, err := q.FindConversation(ctx, models.FindConversationParams{
		WorkspaceID: args.WorkspaceID, ID: args.ConversationID,
	})
	if err != nil {
		return nil, fmt.Errorf("find conversation: %w", err)
	}

	agent, err := q.FindAgent(ctx, conv.WorkspaceID)
	if errors.Is(err, database.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find agent: %w", err)
	}

	ws, err := q.FindWorkspace(ctx, conv.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("find workspace: %w", err)
	}

	e := &env{Workspace: ws, Agent: agent, Conversation: conv, now: r.Clock.Now()}
	if err := r.loadMessages(ctx, q, e, args.TriggerMessageID); err != nil {
		return nil, err
	}
	r.loadConnections(ctx, q, e)
	if err := loadWorkflowStep(ctx, q, e); err != nil {
		return nil, err
	}

	if err := loadProcedures(ctx, q, e); err != nil {
		return nil, err
	}

	return e, r.loadFacts(ctx, q, e)
}

/*
loadMessages loads the trigger by id and the newest public turns,
splitting them into history and the customer messages pending since
the last reply. A run without a trigger message, such as a follow-up
on an action request, loads the turns alone.
*/
func (r *Runner) loadMessages(ctx context.Context, q *models.Queries, e *env, triggerID ids.UUID) error {
	if triggerID != (ids.UUID{}) {
		trigger, err := q.FindConversationMessage(ctx, models.FindConversationMessageParams{
			WorkspaceID: e.Conversation.WorkspaceID, ConversationID: e.Conversation.ID, ID: triggerID,
		})
		if errors.Is(err, database.ErrNotFound) {
			return errTriggerMissing
		}
		if err != nil {
			return fmt.Errorf("find trigger: %w", err)
		}
		e.Trigger = trigger
	}

	latest, err := q.ListLatestPublicMessages(ctx, models.ListLatestPublicMessagesParams{
		WorkspaceID: e.Conversation.WorkspaceID, ConversationID: e.Conversation.ID,
		Now: &e.now, RowLimit: transcriptSize,
	})
	if err != nil {
		return fmt.Errorf("list messages: %w", err)
	}
	slices.Reverse(latest)

	split := len(latest)
	for split > 0 && latest[split-1].AuthorType == string(enums.AuthorTypeContact) {
		split--
	}
	e.History, e.Pending = latest[:split], latest[split:]
	if len(latest) > 0 {
		e.Newest = latest[len(latest)-1].ID
	}

	triggerAt := slices.IndexFunc(latest, func(m models.Message) bool { return m.ID == triggerID })
	if triggerAt >= 0 && triggerAt < split {
		e.ReplyAfter = firstReplyAfter(latest[triggerAt+1:])
	}
	return nil
}

/*
firstReplyAfter names who replied first after the trigger.
*/
func firstReplyAfter(after []models.Message) string {
	for _, m := range after {
		if m.AuthorType != string(enums.AuthorTypeContact) {
			return m.AuthorType
		}
	}
	return ""
}

/*
loadConnections resolves the contact, the channel type and the
connection behind each model role.
*/
func (r *Runner) loadConnections(ctx context.Context, q *models.Queries, e *env) {
	wsID := e.Conversation.WorkspaceID

	if contact, err := q.FindContact(ctx, models.FindContactParams{
		WorkspaceID: wsID, ID: e.Conversation.ContactID,
	}); err == nil {
		e.Contact = &contact
	}

	e.ChannelType = "widget"
	if conn, err := q.FindChannelConnection(ctx, models.FindChannelConnectionParams{
		WorkspaceID: wsID, ID: e.Conversation.ChannelConnectionID,
	}); err == nil {
		e.ChannelType = conn.Type
	}

	e.AnswerKey = keyFor(ctx, q, wsID, e.Agent.AnswerKeyID)
	e.ClassifierKey = e.AnswerKey
	if e.Agent.ClassifierKeyID != nil {
		e.ClassifierKey = keyFor(ctx, q, wsID, e.Agent.ClassifierKeyID)
	}

	e.Version = strv(e.Agent.PromptVersion)
	if r.Prompts != nil {
		e.Version = r.Prompts.Version(e.Version)
		e.Defaults = r.Prompts.Defaults(e.Version)
	}
}

/*
loadFacts reads the spend period, the assignee's presence, how many
teammates are online and whether office hours are open now.
*/
func (r *Runner) loadFacts(ctx context.Context, q *models.Queries, e *env) error {
	wsID := e.Conversation.WorkspaceID

	if capMicros := e.Agent.MonthlySpendCapMicros; capMicros != nil && *capMicros > 0 {
		period, err := q.GetSpendPeriod(ctx, models.GetSpendPeriodParams{
			WorkspaceID: wsID, PeriodStart: database.Date(monthStart(e.now)),
		})
		if err != nil && !errors.Is(err, database.ErrNotFound) {
			return fmt.Errorf("read spend period: %w", err)
		}
		e.Facts.spendCapped = err == nil && (period.CappedAt != nil || period.CostMicros >= *capMicros)
	}

	if member := e.Conversation.AssigneeMemberID; member != nil {
		active, err := q.AssigneeActive(ctx, models.AssigneeActiveParams{
			WorkspaceID: wsID, MemberID: *member, ConversationID: e.Conversation.ID,
			Since: e.now.Add(-24 * time.Hour),
		})
		if err != nil {
			return fmt.Errorf("read assignee presence: %w", err)
		}
		e.Facts.assigneeActive = active
	}

	online, err := q.CountOnlineMembers(ctx, wsID)
	if err != nil {
		return fmt.Errorf("count online members: %w", err)
	}
	e.Facts.onlineMembers = online
	e.Facts.withinHours, e.Facts.hoursErr = officehours.Within(e.Agent.OfficeHours, e.now)
	e.Triage, err = loadTriage(ctx, q, e)
	return err
}

/*
keyFor loads a role's connection, nil when unset or gone.
*/
func keyFor(ctx context.Context, q *models.Queries, workspaceID ids.UUID, keyID *ids.UUID) *models.AiProviderKey {
	if keyID == nil {
		return nil
	}
	key, err := q.FindProviderKey(ctx, models.FindProviderKeyParams{WorkspaceID: workspaceID, ID: *keyID})
	if err != nil {
		return nil
	}
	return &key
}

/*
monthStart is the first instant of the UTC month holding t, the spend
period's key.
*/
func monthStart(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

/*
included lists the ids of every message the run read.
*/
func (e *env) included() []ids.UUID {
	out := make([]ids.UUID, 0, len(e.History)+len(e.Pending))
	for _, m := range append(append([]models.Message{}, e.History...), e.Pending...) {
		if m.ID != (ids.UUID{}) {
			out = append(out, m.ID)
		}
	}
	return out
}

/*
pendingText joins the customer's messages since the last reply.
*/
func (e *env) pendingText() string {
	var parts []string
	for _, m := range e.Pending {
		if text := body(m); text != "" {
			parts = append(parts, text)
		}
	}
	return joinParagraphs(parts)
}
