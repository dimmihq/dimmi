/*
|--------------------------------------------------------------------------
| Workflow Fields
|--------------------------------------------------------------------------
|
| Workflows read everything an audience can, plus what only exists once
| there is a conversation: its channel, status, priority, tags and
| owner, the message that triggered the run, the rating, what dimmi did,
| whether the team is open, and the run's own variables. Fields that
| belong to one kind of trigger say so, and a rule that reads one under
| another trigger is refused before it can silently never match.
|
*/

package workflowspec

import (
	"slices"

	"github.com/dimmihq/dimmi/app/support/audience"
)

/*
Catalog returns the fields workflow rules may read.
*/
func Catalog() *audience.Catalog {
	fields := slices.Concat(conversationFields(), eventFields(), ticketFields(), ticketEventFields())

	return audience.Base().With(fields...)
}

/*
conversationFields are facts of the conversation itself.
*/
func conversationFields() []audience.Field {
	f := func(key string, t audience.FieldType, options ...string) audience.Field {
		return audience.Field{
			Key: key, Type: t, Where: audience.Conversation, Trust: audience.Observed, Options: options,
		}
	}

	return []audience.Field{
		f("conversation.channel_type", audience.TypeList),
		f("conversation.inbox_id", audience.TypeID),
		f("conversation.status", audience.TypeList, "open", "snoozed", "closed"),
		f("conversation.priority", audience.TypeList, "none", "low", "medium", "high", "urgent"),
		f("conversation.tags", audience.TypeSet),
		f("conversation.assignee_id", audience.TypeID),
		f("conversation.team_id", audience.TypeID),
		f("conversation.ai_state", audience.TypeList),
		f("conversation.subject", audience.TypeString),
		f("conversation.message_count", audience.TypeNumber),
		f("conversation.waiting_minutes", audience.TypeNumber),
		f("conversation.created_at", audience.TypeDate),
		f("conversation.is_test", audience.TypeBool),
		f("office_hours.open", audience.TypeBool),
		f("time.weekday", audience.TypeList, "mon", "tue", "wed", "thu", "fri", "sat", "sun"),
		{
			Key: "run.vars.", Prefix: true, Type: audience.TypeAuto, Where: audience.Conversation,
			Trust: audience.Claimed,
		},
	}
}

/*
eventFields are facts of the event that started the run, each readable
only under the triggers that carry it.
*/
func eventFields() []audience.Field {
	messages := []string{
		TriggerFirstMessage, TriggerCustomerReopened, TriggerCustomerMessage, TriggerTeammateMessage,
	}
	on := func(key string, t audience.FieldType, trust audience.Trust, triggers ...string) audience.Field {
		return audience.Field{Key: key, Type: t, Where: audience.Conversation, Trust: trust, Triggers: triggers}
	}

	return []audience.Field{
		on("message.body", audience.TypeString, audience.Claimed, messages...),
		on("message.has_attachments", audience.TypeBool, audience.Observed, messages...),
		on("note.body", audience.TypeString, audience.Observed, TriggerNoteCreated),
		on("rating.score", audience.TypeNumber, audience.Claimed, TriggerRated),
		on("dimmi.handover_reason", audience.TypeList, audience.Observed, TriggerDimmiHandedOver),
		on("trigger.to", audience.TypeList, audience.Observed, TriggerStatusChanged, TriggerAssignmentChanged),
		on("trigger.from", audience.TypeList, audience.Observed, TriggerStatusChanged),
		on("trigger.reason", audience.TypeString, audience.Observed, TriggerStatusChanged),
		on("trigger.to_member", audience.TypeID, audience.Observed, TriggerAssignmentChanged),
		on("trigger.to_team", audience.TypeID, audience.Observed, TriggerAssignmentChanged),
	}
}

/*
ruleScope says where a rule sits: the workflow's audience, or a
condition inside the run.
*/
type ruleScope string

/*
The rule scopes.
*/
const (
	scopeAudience ruleScope = "audience"
	scopeBranch   ruleScope = "branch"
)

/*
fieldAllowed reports whether a rule under the trigger may read the
field. Conversation fields need a conversation, which an opener never
has; run variables only exist inside a run, never in an audience.
*/
func fieldAllowed(f audience.Field, t Trigger, scope ruleScope) bool {
	if len(f.Triggers) > 0 && !slices.Contains(f.Triggers, t.Key) {
		return false
	}

	if f.Key == "run.vars." {
		/*
			A run's own variables belong to the run, not to the conversation
			it works on: a step put them there a moment ago. So a branch may
			read them under any trigger, an opener included, while an
			audience, which is decided before any run exists, may not.
		*/
		return scope != scopeAudience
	}

	if f.Where == audience.Conversation {
		return t.ConversationFields
	}

	return f.Where != audience.Client || t.Key == TriggerOpener
}

/*
leaves returns every condition of a rule tree.
*/
func leaves(r *audience.Rule) []audience.Rule {
	if r == nil {
		return nil
	}

	if r.IsLeaf() {
		return []audience.Rule{*r}
	}

	var out []audience.Rule
	for _, child := range slices.Concat(r.All, r.Any) {
		out = append(out, leaves(&child)...)
	}

	return append(out, leaves(r.Not)...)
}
