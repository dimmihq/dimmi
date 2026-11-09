/*
|--------------------------------------------------------------------------
| Applying Conversation Changes
|--------------------------------------------------------------------------
|
| Conversation operations go through the same change the inbox makes,
| so assignment rules, timeline entries and live updates all follow.
| Each one keeps the value it replaced; a revert puts it back only while
| the field still holds what Operator wrote.
|
*/

package operator

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/messages"
	"github.com/dimmihq/dimmi/app/actions/tags"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
)

/*
assignment is a conversation's assignee and team, as kept before and
after an assignment.
*/
type assignment struct {
	Member *ids.UUID `json:"member_id"`
	Team   *ids.UUID `json:"team_id"`
}

/*
applyOp applies one operation and records what it replaced.
*/
func (e *Engine) applyOp(ctx context.Context, m authz.Membership, op *Operation) error {
	if GroupOf(op.Kind) != GroupConversations && GroupOf(op.Kind) != GroupNotes {
		return e.applyContent(ctx, m, op)
	}
	before, err := e.Services.Show.Handle(ctx, conversations.ShowConversationInput{
		Membership: m, Ref: conversations.Ref{ID: *op.Conversation},
	})
	if err != nil {
		return err
	}
	if op.Kind == KindNote {
		written, err := e.Services.Note.Handle(ctx, messages.NoteInput{
			Membership: m, Ref: conversations.Ref{ID: *op.Conversation}, Body: deref(op.Value),
		})
		if err == nil {
			op.Created, op.Outcome = &written.Message.ID, OutcomeApplied
		}
		return err
	}
	changes, was := forward(*op, before)
	after, err := e.change(ctx, m, *op.Conversation, changes)
	if err != nil {
		return err
	}
	op.Before, op.After, op.Outcome = rawJSON(was), rawJSON(stateOf(op.Kind, after, was)), OutcomeApplied
	return nil
}

/*
forward is the change an operation makes and the state it replaces.
*/
func forward(op Operation, c conversations.Loaded) (conversations.Changes, any) {
	switch op.Kind {
	case KindTag:
		return conversations.Changes{AddTags: tags.Refs{Names: op.Tags}}, tagNamesOf(c)
	case KindUntag:
		return conversations.Changes{RemoveTags: tagIDs(c, op.Tags)}, tagNamesOf(c)
	case KindAssign:
		return conversations.Changes{
			AssignMember: op.Member != nil || op.Unassign, AssignTeam: op.Team != nil || op.Unassign,
			Assign: conversations.Assignment{Member: op.Member, Team: op.Team},
		}, assignmentOf(c)
	case KindPriority:
		p := enums.ConversationPriority(deref(op.Value))
		return conversations.Changes{Attributes: conversations.Attributes{Priority: &p}},
			string(enums.ConversationPriorityOf(c.Conversation.Priority))
	case KindStatus:
		return conversations.Changes{Status: op.Value}, c.Conversation.Status
	default:
		return conversations.Changes{Attributes: conversations.Attributes{Subject: op.Value}},
			deref(c.Conversation.Subject)
	}
}

/*
stateOf is the state an operation left: for tagging, the tags it added
or removed; otherwise the field as it now is.
*/
func stateOf(k Kind, c conversations.Loaded, was any) any {
	switch k {
	case KindTag, KindUntag:
		before, _ := was.([]string)
		return symmetric(before, tagNamesOf(c))
	case KindAssign:
		return assignmentOf(c)
	case KindPriority:
		return string(enums.ConversationPriorityOf(c.Conversation.Priority))
	case KindStatus:
		return c.Conversation.Status
	default:
		return deref(c.Conversation.Subject)
	}
}

/*
revertOp undoes one applied operation, skipping it when the field moved
on since.
*/
func (e *Engine) revertOp(ctx context.Context, m authz.Membership, op *Operation) error {
	if op.Outcome != OutcomeApplied {
		return nil
	}
	if GroupOf(op.Kind) != GroupConversations && GroupOf(op.Kind) != GroupNotes {
		return e.revertContent(ctx, m, op)
	}
	ref := conversations.Ref{ID: *op.Conversation}
	if op.Kind == KindNote {
		_, err := e.Services.Delete.Handle(ctx, messages.UndoInput{Membership: m, Ref: ref, MessageID: *op.Created})
		op.Outcome = OutcomeReverted
		return err
	}
	current, err := e.Services.Show.Handle(ctx, conversations.ShowConversationInput{Membership: m, Ref: ref})
	if err != nil {
		return err
	}
	changes, ok := backward(*op, current)
	if !ok {
		op.Outcome = OutcomeSkipped
		return nil
	}
	if _, err := e.change(ctx, m, *op.Conversation, changes); err != nil {
		return err
	}
	op.Outcome = OutcomeReverted
	return nil
}

/*
backward is the change that undoes an operation, and false when the
field no longer holds what the operation wrote.
*/
func backward(op Operation, c conversations.Loaded) (conversations.Changes, bool) {
	switch op.Kind {
	case KindTag:
		return conversations.Changes{RemoveTags: tagIDs(c, stringList(op.After))}, true
	case KindUntag:
		return conversations.Changes{AddTags: tags.Refs{Names: stringList(op.After)}}, true
	case KindAssign:
		var was, wrote assignment
		_ = json.Unmarshal(op.Before, &was)
		_ = json.Unmarshal(op.After, &wrote)
		if !sameAssignment(assignmentOf(c), wrote) {
			return conversations.Changes{}, false
		}
		return conversations.Changes{
			AssignMember: true, AssignTeam: true,
			Assign: conversations.Assignment{Member: was.Member, Team: was.Team},
		}, true
	}
	was, wrote := text(op.Before), text(op.After)
	if stateOf(op.Kind, c, nil) != wrote {
		return conversations.Changes{}, false
	}
	switch op.Kind {
	case KindPriority:
		p := enums.ConversationPriority(was)
		return conversations.Changes{Attributes: conversations.Attributes{Priority: &p}}, true
	case KindStatus:
		if was == "snoozed" {
			was = "open"
		}
		return conversations.Changes{Status: &was}, true
	default:
		return conversations.Changes{Attributes: conversations.Attributes{Subject: &was}}, true
	}
}

/*
change applies changes to a conversation as the teammate.
*/
func (e *Engine) change(ctx context.Context, m authz.Membership, id ids.UUID, c conversations.Changes,
) (conversations.Loaded, error) {
	applied, err := e.Services.Change.Handle(ctx, conversations.ApplyChangesInput{
		Membership: m, Ref: conversations.Ref{ID: id}, Changes: c,
	})
	return applied.Conversation, err
}

/*
tagNamesOf lists a conversation's tag names.
*/
func tagNamesOf(c conversations.Loaded) []string {
	out := make([]string, len(c.Tags))
	for i, t := range c.Tags {
		out[i] = t.Name
	}
	return out
}

/*
tagIDs finds the ids of the named tags on a conversation.
*/
func tagIDs(c conversations.Loaded, names []string) []ids.UUID {
	var out []ids.UUID
	for _, t := range c.Tags {
		if slices.ContainsFunc(names, func(n string) bool { return strings.EqualFold(n, t.Name) }) {
			out = append(out, t.ID)
		}
	}
	return out
}

/*
assignmentOf reads a conversation's assignment.
*/
func assignmentOf(c conversations.Loaded) assignment {
	return assignment{Member: c.Conversation.AssigneeMemberID, Team: c.Conversation.AssigneeTeamID}
}

/*
sameAssignment compares two assignments.
*/
func sameAssignment(a, b assignment) bool {
	return sameID(a.Member, b.Member) && sameID(a.Team, b.Team)
}

/*
sameID compares two optional ids.
*/
func sameID(a, b *ids.UUID) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

/*
symmetric lists the names in exactly one of two lists.
*/
func symmetric(a, b []string) []string {
	var out []string
	for _, n := range a {
		if !slices.Contains(b, n) {
			out = append(out, n)
		}
	}
	for _, n := range b {
		if !slices.Contains(a, n) {
			out = append(out, n)
		}
	}
	return out
}

/*
stringList reads a stored list of names.
*/
func stringList(raw json.RawMessage) []string {
	var out []string
	_ = json.Unmarshal(raw, &out)
	return out
}

/*
text reads a stored text value.
*/
func text(raw json.RawMessage) string {
	var out string
	_ = json.Unmarshal(raw, &out)
	return out
}
