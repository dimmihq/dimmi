/*
|--------------------------------------------------------------------------
| Workspace Actions
|--------------------------------------------------------------------------
|
| The workspace connects its own systems once, as actions, and chooses
| for each whether dimmi, Operator or both may use it. Operator offers
| the published actions given to it as tools. A lookup runs at once, as
| the teammate, and its result is untrusted data. A change never runs
| from the chat: it becomes a changeset, and applying it is the
| teammate's approval; an action that needs an admin's approval needs
| an admin to apply it. Nothing an action did can be undone from here.
| In a thread about a conversation, actions read that customer's
| details; elsewhere, only actions that need no customer are offered.
|
*/

package operator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/ai/actionkit"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
actionPrefix starts the tool name of every workspace action, so an
action never takes the name of one of Operator's own tools.
*/
const actionPrefix = "action_"

/*
offerActions loads the published actions Operator may use in this run.
A failure costs only the actions: Operator still answers.
*/
func (e *Engine) offerActions(ctx context.Context, s *session) {
	if e.Services.Actions == nil || s.settings.PolicyFor(GroupActions) == PolicyOff {
		return
	}
	identity := actionkit.Identity{}
	if s.conversation != nil {
		if c, err := e.contact(ctx, s.m, s.conversation.ContactID); err == nil {
			identity = actionkit.Identity{Verified: true, Contact: &c}
		}
	}
	offered, err := e.Services.Actions.Offer(ctx, actionkit.OfferInput{
		WorkspaceID: s.m.Workspace.ID, Identity: identity, Agent: actionkit.AgentOperator,
	})
	if err == nil {
		s.actions, s.identity = offered, identity
	}
}

/*
actionTools are the session's workspace actions as tools.
*/
func (e *Engine) actionTools(s *session) []tool {
	out := make([]tool, 0, len(s.actions))
	for _, live := range s.actions {
		t := tool{spec: contracts.ToolSpec{
			Name: actionPrefix + live.Spec.Name, Description: actionDescription(live.Spec),
			Schema: live.Spec.ModelSchema(),
		}, handle: e.lookup(live)}
		if live.Spec.SideEffect == actionkit.SideEffectWrite {
			t.writes, t.handle = []Kind{KindAction}, e.proposeAction(live)
		}
		out = append(out, t)
	}
	return out
}

/*
actionDescription tells the model what an action does and how calling
it works for Operator.
*/
func actionDescription(s actionkit.Spec) string {
	if s.SideEffect == actionkit.SideEffectRead {
		return s.Description + " (Read-only lookup in the workspace's own systems.)"
	}
	return s.Description + " (Changes data in the workspace's own systems. Calling it proposes the change as a " +
		"changeset; nothing runs until the teammate applies it, and it cannot be undone afterwards.)"
}

/*
lookup runs a read action at once, as the teammate.
*/
func (e *Engine) lookup(live actionkit.Live) handler {
	return func(ctx context.Context, s *session, args json.RawMessage) (result, error) {
		reply, err := e.Services.Actions.Call(ctx, e.callInput(s, live, args))
		if err != nil {
			return result{}, err
		}
		s.tainted = true
		st := Step{Tool: actionPrefix + live.Spec.Name, Summary: live.Spec.Title}
		return result{text: reply.Text, step: st}, nil
	}
}

/*
callInput is the call an action makes in this session: in the thread's
conversation when it has one, as the teammate through Operator.
*/
func (e *Engine) callInput(s *session, live actionkit.Live, args json.RawMessage) actionkit.CallInput {
	in := actionkit.CallInput{
		WorkspaceID: s.m.Workspace.ID, RunID: s.run.ID, Live: live, Args: args, Identity: s.identity,
		Agent: conversations.Actor{Type: "member", ID: s.m.Member.ID, Name: cmpName(s) + " via Operator"},
	}
	if s.conversation != nil {
		in.ConversationID, in.ContactID = s.conversation.ID, s.conversation.ContactID
	}
	return in
}

/*
cmpName is how calls name the teammate.
*/
func cmpName(s *session) string {
	if s.askerName == "" {
		return "A teammate"
	}
	return s.askerName
}

/*
proposeAction turns a write action into a changeset the teammate
applies.
*/
func (e *Engine) proposeAction(live actionkit.Live) handler {
	return func(ctx context.Context, s *session, args json.RawMessage) (result, error) {
		input, err := actionkit.Check(live.Spec, args)
		if err != nil {
			return result{}, errors.Join(errBadArguments, err)
		}
		op := Operation{
			Kind: KindAction, Action: &live.ActionID, Version: &live.VersionID, Definition: args,
			Label: truncate(actionkit.Summarize(live.Spec, input), 200),
		}
		if s.conversation != nil {
			op.Conversation, op.Number = &s.conversation.ID, s.conversation.Number
		}
		asked := proposal{Title: live.Spec.Title, Reason: "Requested in the chat."}
		in := changeset{proposal: asked, ops: []Operation{op}}
		row, err := e.createChangeset(ctx, s, in)
		if err != nil {
			return result{}, err
		}
		s.proposed = append(s.proposed, row)
		public := models.PublicID(models.PrefixOperatorChangeset, row.ID)
		return result{
			text: fmt.Sprintf("Changeset %s waits for the teammate to apply %s.", public, live.Spec.Title),
			step: Step{Tool: actionPrefix + live.Spec.Name, Summary: "Proposed: " + live.Spec.Title, Changeset: public},
		}, nil
	}
}

/*
runAction runs an action a teammate approved by applying its changeset,
refusing when the action changed since it was proposed or needs an
admin's approval the teammate cannot give.
*/
func (e *Engine) runAction(ctx context.Context, m authz.Membership, op *Operation) error {
	in := actionkit.CallInput{
		WorkspaceID: m.Workspace.ID, Args: op.Definition, ApprovedBy: &m.Member.ID,
		Agent: conversations.Actor{Type: "member", ID: m.Member.ID, Name: "Operator"},
	}
	if op.Conversation != nil {
		c, err := e.Services.Show.Handle(ctx, conversations.ShowConversationInput{
			Membership: m, Ref: conversations.Ref{ID: *op.Conversation},
		})
		if err != nil {
			return err
		}
		in.ConversationID, in.ContactID = c.Conversation.ID, c.Contact.ID
		in.Identity = actionkit.Identity{Verified: true, Contact: &c.Contact}
	}
	live, err := e.liveAction(ctx, m, op, in.Identity)
	if err != nil {
		return err
	}
	if live.Spec.Approval == actionkit.ApprovalAdmin && !m.Can("ai_actions:manage") {
		return exceptions.ErrForbidden
	}
	in.Live = live
	reply, err := e.Services.Actions.Call(ctx, in)
	if err != nil {
		return err
	}
	if reply.Call != nil {
		op.Created = &reply.Call.ID
	}
	if !reply.Succeeded {
		return fmt.Errorf("%w: %s", errActionFailed, strings.TrimSpace(truncate(reply.Text, 300)))
	}
	op.After = rawJSON(truncate(reply.Text, 2000))
	return nil
}

/*
errActionFailed marks an action that ran and did not succeed.
*/
var errActionFailed = errors.New("the action did not succeed")

/*
liveAction finds the published version a changeset proposed, refusing
an action that was changed, unpublished or taken from Operator since.
*/
func (e *Engine) liveAction(ctx context.Context, m authz.Membership, op *Operation, identity actionkit.Identity,
) (actionkit.Live, error) {
	if e.Services.Actions == nil || op.Action == nil || op.Version == nil {
		return actionkit.Live{}, exceptions.ErrNotFound
	}
	var version models.AiActionVersion
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		version, err = models.New(tx).FindActionVersion(ctx, models.FindActionVersionParams{
			WorkspaceID: m.Workspace.ID, ID: *op.Version,
		})
		return err
	})
	if errors.Is(err, database.ErrNotFound) {
		return actionkit.Live{}, errActionChanged
	}
	if err != nil {
		return actionkit.Live{}, fmt.Errorf("find action version: %w", err)
	}
	offered, err := e.Services.Actions.Offer(ctx, actionkit.OfferInput{
		WorkspaceID: m.Workspace.ID, Agent: actionkit.AgentOperator, Identity: identity,
	})
	if err != nil {
		return actionkit.Live{}, fmt.Errorf("offer actions: %w", err)
	}
	for _, live := range offered {
		if live.VersionID == version.ID {
			return live, nil
		}
	}
	return actionkit.Live{}, errActionChanged
}

/*
errActionChanged refuses to run an action that changed since it was
proposed.
*/
var errActionChanged = fmt.Errorf("%w: the action changed or is no longer given to Operator; ask again",
	errBadArguments)
