/*
|--------------------------------------------------------------------------
| Calls From The Agent
|--------------------------------------------------------------------------
|
| The model asks for an action by name with arguments. The arguments are
| checked against the schema and frozen together with the customer
| values the action binds. A read, or a write nobody has to agree to,
| runs at once and its result goes back to the model as untrusted data.
| A write that needs the customer's confirmation or a teammate's
| approval does not block the conversation: the call waits in its own
| row, the model hears it is pending and tells the customer, and the
| agent is woken again once there is news. Nothing the model writes
| describes what will run; the summary a customer confirms is rendered
| here from the validated arguments.
|
*/

package actionkit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/actionschema"
)

/*
summaryValue caps one value in a rendered summary.
*/
const summaryValue = 80

/*
CallInput is one action the model asked for in a conversation run:
where, by whom, the offered action, the provider's tool call id, the
raw arguments, and who the customer is. ApprovedBy names the teammate
who already agreed to a write, as when they apply a change Operator
proposed; such a call runs at once, recorded as their decision.
*/
type CallInput struct {
	WorkspaceID    ids.UUID
	ConversationID ids.UUID
	ContactID      ids.UUID
	RunID          ids.UUID
	Live           Live
	ToolCallID     string
	Args           json.RawMessage
	Identity       Identity
	Agent          conversations.Actor
	ApprovedBy     *ids.UUID
}

/*
CallReply is what came of it: the text the model reads, the call row
when one was written, and whether it succeeded, now waits for the
customer's confirmation, or waits for a teammate.
*/
type CallReply struct {
	Text      string
	Call      *models.AiActionCall
	Succeeded bool
	Awaiting  bool
	Pending   bool
}

/*
Call handles one action the model asked for.
*/
func (k *Kit) Call(ctx context.Context, in CallInput) (CallReply, error) {
	s := in.Live.Spec
	input, refused := validate(s, in.Args)
	if refused != nil {
		return *refused, nil
	}
	contact := ContactValues(in.Identity)
	if s.UsesContact() && !bindable(s, contact) && awaitsEmail(s, in.Identity) {
		return k.awaitEmail(ctx, in, input)
	}
	if s.UsesContact() && !bindable(s, contact) {
		return refusal(s, "identity_unavailable", "not available: the customer is not signed in"), nil
	}
	bindings := bindingsFor(s, contact)

	if s.NeedsAgreement() && in.ApprovedBy == nil {
		return k.propose(ctx, in, input, bindings)
	}
	return k.runNow(ctx, in, input, bindings)
}

/*
Check validates a model's arguments for an action without calling it,
returning them as the call would read them, or the reason they do not
fit.
*/
func Check(s Spec, args json.RawMessage) (map[string]any, error) {
	schema, err := actionschema.Compile(s.ModelSchema())
	if err != nil {
		return nil, err
	}
	return schema.Validate(args)
}

/*
validate checks the model's arguments against the schema the model was
shown, which leaves out the arguments the admin bound, returning the
refusal the model reads when they do not fit.
*/
func validate(s Spec, args json.RawMessage) (map[string]any, *CallReply) {
	schema, err := actionschema.Compile(s.ModelSchema())
	if err == nil {
		var input map[string]any
		if input, err = schema.Validate(args); err == nil {
			return input, nil
		}
	}
	reply := refusal(s, "invalid_arguments", err.Error())
	return nil, &reply
}

/*
runNow records the call as running, sends it and records how it ended.
*/
func (k *Kit) runNow(ctx context.Context, in CallInput, input map[string]any, b Bindings) (CallReply, error) {
	s := in.Live.Spec
	var call models.AiActionCall
	var target Target
	err := k.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		row, err := k.insert(ctx, tx, in, newCall{input: input, bindings: b, state: "running", sent: true})
		if err != nil {
			return err
		}
		call = row
		target, err = k.target(ctx, tx, call, s)
		return err
	})
	if err != nil {
		return CallReply{}, err
	}

	out := k.Send(ctx, target)
	finished, err := k.finish(ctx, finishing{call: call, out: out, actor: in.Agent})
	if err != nil {
		return CallReply{}, err
	}
	return CallReply{Text: wrap(s, finished, out.Result), Call: &finished, Succeeded: out.Succeeded}, nil
}

/*
propose records a write that must wait for the customer or a teammate,
canceling any older confirmation still waiting in the conversation.
*/
func (k *Kit) propose(ctx context.Context, in CallInput, input map[string]any, b Bindings) (CallReply, error) {
	s := in.Live.Spec
	now := k.Clock.Now()
	nc := newCall{input: input, bindings: b, state: "awaiting_confirmation", expires: now.Add(ConfirmationTTL)}
	if s.Confirmation == ConfirmationNone {
		nc.state, nc.expires = "pending_approval", now.Add(approvalTTL(s))
	}

	var call models.AiActionCall
	err := k.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		if err := k.supersede(ctx, tx, in); err != nil {
			return err
		}
		row, err := k.insert(ctx, tx, in, nc)
		if err != nil {
			return err
		}
		call = row
		if err := k.note(ctx, tx, step{call, in.Agent, EventRequested}); err != nil {
			return err
		}
		if call.State == "pending_approval" {
			if err := k.askApproval(ctx, tx, call); err != nil {
				return err
			}
		}
		return k.scheduleExpiry(ctx, tx, call)
	})
	if err != nil {
		return CallReply{}, err
	}
	return CallReply{
		Text: pendingText(s, call), Call: &call, Awaiting: call.State == "awaiting_confirmation",
		Pending: call.State == "pending_approval",
	}, nil
}

/*
newCall is the starting state of a call row.
*/
type newCall struct {
	input    map[string]any
	bindings Bindings
	state    string
	sent     bool
	expires  time.Time
}

/*
insert writes the call row and counts it on the run.
*/
func (k *Kit) insert(ctx context.Context, tx *database.Tx, in CallInput, nc newCall) (models.AiActionCall, error) {
	s, now := in.Live.Spec, k.Clock.Now()
	input, _ := json.Marshal(nc.input)
	bindings, _ := json.Marshal(nc.bindings)
	params := models.CreateActionCallParams{
		WorkspaceID: in.WorkspaceID, ActionID: in.Live.ActionID, ActionVersionID: &in.Live.VersionID,
		ConnectionID: s.ConnectionID, Mode: "live", SideEffect: s.SideEffect, Confirmation: s.Confirmation,
		Approval: s.Approval, State: nc.state, RunID: nilID(in.RunID), ConversationID: nilID(in.ConversationID),
		ContactID: nilID(in.ContactID), ToolCallID: nilString(in.ToolCallID), Title: s.Title,
		Summary: Summarize(s, nc.input), Input: input, Bindings: bindings, Now: now,
	}
	if nc.sent {
		params.SentAt, params.Attempts = &now, 1
	}
	if in.ApprovedBy != nil {
		params.DecidedByMemberID, params.DecidedAt = in.ApprovedBy, &now
	}
	if !nc.expires.IsZero() {
		params.ExpiresAt = &nc.expires
	}
	q := models.New(tx)
	call, err := q.CreateActionCall(ctx, params)
	if err != nil {
		return call, fmt.Errorf("record action call: %w", err)
	}
	if in.RunID != (ids.UUID{}) {
		if err := q.CountRunActionCall(ctx, models.CountRunActionCallParams{
			WorkspaceID: in.WorkspaceID, ID: in.RunID, Now: now,
		}); err != nil {
			return call, fmt.Errorf("count action call: %w", err)
		}
	}
	return call, nil
}

/*
supersede cancels the confirmations still waiting in the conversation:
a customer has at most one card to answer.
*/
func (k *Kit) supersede(ctx context.Context, tx *database.Tx, in CallInput) error {
	old, err := models.New(tx).SupersedeConversationConfirmations(ctx,
		models.SupersedeConversationConfirmationsParams{
			WorkspaceID: in.WorkspaceID, ConversationID: nilID(in.ConversationID), Now: k.Clock.Now(),
		})
	if err != nil {
		return fmt.Errorf("supersede confirmations: %w", err)
	}
	for _, call := range old {
		if err := k.note(ctx, tx, step{call, in.Agent, EventCanceled}); err != nil {
			return err
		}
	}
	return nil
}

/*
askApproval raises the event that tells the team a call waits for them.
*/
func (k *Kit) askApproval(ctx context.Context, tx *database.Tx, call models.AiActionCall) error {
	if call.ConversationID == nil || call.ExpiresAt == nil {
		return nil
	}
	return k.Events.Dispatch(ctx, tx, appevents.AiActionCallApprovalRequested{
		Base: appevents.Base{
			ID: ids.New(), WorkspaceID: call.WorkspaceID, OccurredAt: k.Clock.Now(),
			Actor: appevents.Actor{Type: "agent"},
		},
		ConversationRef: appevents.ConversationRef{ConversationID: *call.ConversationID},
		CallID:          call.ID, Title: call.Title, Summary: call.Summary, Approval: call.Approval,
		ExpiresAt: *call.ExpiresAt,
	})
}

/*
scheduleExpiry queues the job that ends the call at its deadline.
*/
func (k *Kit) scheduleExpiry(ctx context.Context, tx *database.Tx, call models.AiActionCall) error {
	if call.ExpiresAt == nil || k.Queue == nil {
		return nil
	}
	return k.Queue.PushTx(ctx, tx, ExpireCallArgs{
		WorkspaceID: call.WorkspaceID, CallID: call.ID, At: call.ExpiresAt.UnixMilli(),
	}, queue.At(*call.ExpiresAt))
}

/*
Summarize renders what a call would do, from the action's title and the
validated arguments, for the confirmation card and the approval: never
from anything the model wrote about it.
*/
func Summarize(s Spec, input map[string]any) string {
	parts := make([]string, 0, len(input))
	for _, k := range sortedKeys(input) {
		v, err := json.Marshal(input[k])
		if err != nil {
			continue
		}
		text := strings.Trim(string(v), `"`)
		parts = append(parts, strings.ReplaceAll(k, "_", " ")+": "+truncateText(text, summaryValue))
	}
	if len(parts) == 0 {
		return s.Title
	}
	return s.Title + ": " + strings.Join(parts, ", ")
}

/*
approvalTTL is how long a teammate has to decide.
*/
func approvalTTL(s Spec) time.Duration {
	return time.Duration(s.ApprovalTimeoutMinutes) * time.Minute
}

/*
nilID maps a zero id to NULL.
*/
func nilID(id ids.UUID) *ids.UUID {
	if id == (ids.UUID{}) {
		return nil
	}
	return &id
}

/*
nilString maps an empty string to NULL.
*/
func nilString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
