/*
|--------------------------------------------------------------------------
| Which Events Notify
|--------------------------------------------------------------------------
|
| The inbox's events, read as notification triggers. A conversation's
| first customer message is its "new conversation", so the alert can show
| what the customer wrote; later customer messages are replies. Notes
| notify the members they mention, edits only the ones newly mentioned.
| A conversation reassigned to the same person, or a snooze ended by hand,
| tells nobody.
|
*/

package notifications

import (
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	appevents "github.com/dimmihq/dimmi/app/events"
)

/*
actor reads the event's actor into the alert's fields.
*/
func actor(b appevents.Base) (string, *ids.UUID) {
	if b.Actor.ID == (ids.UUID{}) {
		return b.Actor.Type, nil
	}
	id := b.Actor.ID
	return b.Actor.Type, &id
}

/*
base fills what every event shares.
*/
func base(trigger string, b appevents.Base, conversation *ids.UUID) Event {
	kind, id := actor(b)
	return Event{
		Trigger: trigger, WorkspaceID: b.WorkspaceID, ConversationID: conversation, ActorType: kind, ActorID: id,
		OccurredAt: b.OccurredAt,
	}
}

/*
FromMessageCreated reads a new message: a customer starting or replying,
or a note mentioning members.
*/
func FromMessageCreated(e appevents.MessageCreated) (Event, bool) {
	conv, msg := e.ConversationID, e.MessageID
	switch {
	case e.Type == "message" && e.AuthorType == "contact":
		trigger := TriggerCustomerReplied
		if e.ConversationCreated {
			trigger = TriggerNewConversation
		}
		ev := base(trigger, e.Base, &conv)
		ev.MessageID = &msg
		return ev, true
	case e.Type == "note" && len(e.MentionedMemberIDs) > 0:
		ev := base(TriggerMentioned, e.Base, &conv)
		ev.MessageID, ev.Targets = &msg, e.MentionedMemberIDs
		return ev, true
	}
	return Event{}, false
}

/*
FromMessageUpdated reads a note edit that mentions new members.
*/
func FromMessageUpdated(e appevents.MessageUpdated) (Event, bool) {
	if len(e.AddedMentionedMemberIDs) == 0 {
		return Event{}, false
	}
	conv, msg := e.ConversationID, e.MessageID
	ev := base(TriggerMentioned, e.Base, &conv)
	ev.MessageID, ev.Targets = &msg, e.AddedMentionedMemberIDs
	return ev, true
}

/*
FromAssigned reads a conversation given to a member.
*/
func FromAssigned(e appevents.ConversationAssigned) (Event, bool) {
	if e.ToMemberID == nil || (e.FromMemberID != nil && *e.FromMemberID == *e.ToMemberID) {
		return Event{}, false
	}
	conv := e.ConversationID
	ev := base(TriggerAssigned, e.Base, &conv)
	ev.Targets = []ids.UUID{*e.ToMemberID}
	return ev, true
}

/*
FromHandoff reads the agent handing a conversation to the team.
*/
func FromHandoff(e appevents.AiHandoffRequested) Event {
	conv := e.ConversationID
	return base(TriggerHandoff, e.Base, &conv)
}

/*
FromApprovalRequested reads an action call waiting for a teammate.
*/
func FromApprovalRequested(e appevents.AiActionCallApprovalRequested) Event {
	conv, call := e.ConversationID, e.CallID
	ev := base(TriggerApprovalRequired, e.Base, &conv)
	ev.CallID, ev.Approval = &call, e.Approval
	return ev
}

/*
FromUnsnoozed reads a snooze that ran out.
*/
func FromUnsnoozed(e appevents.ConversationUnsnoozed) (Event, bool) {
	if e.Reason != "expired" {
		return Event{}, false
	}
	conv := e.ConversationID
	return base(TriggerSnoozeExpired, e.Base, &conv), true
}

/*
FromTeamMemberAdded reads a member added to a team by someone else.
*/
func FromTeamMemberAdded(e appevents.TeamMemberAdded, now time.Time) (Event, bool) {
	if e.AddedByMemberID == e.MemberID {
		return Event{}, false
	}
	team, by := e.TeamID, e.AddedByMemberID
	return Event{
		Trigger: TriggerTeamJoined, WorkspaceID: e.WorkspaceID, TeamID: &team, ActorType: "member", ActorID: &by,
		Targets: []ids.UUID{e.MemberID}, OccurredAt: now,
	}, true
}
