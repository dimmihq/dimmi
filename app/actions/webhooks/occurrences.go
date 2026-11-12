/*
|--------------------------------------------------------------------------
| Event Occurrences
|--------------------------------------------------------------------------
|
| Every domain event the catalog lists becomes one occurrence: its id,
| its time, who caused it, the object its payload shows, and the few
| facts of the event a snapshot cannot tell, such as what an assignment
| changed from and to. Ids leave in their public form. An event that
| carries no id of its own gets a new one here, once, so every endpoint
| sees the same id for it.
|
*/

package webhooks

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Subjects an occurrence's payload can show.
*/
const (
	subjectConversation = "conversation"
	subjectMessage      = "message"
	subjectContact      = "contact"
	subjectTag          = "tag"
	subjectContactEvent = "contact_event"
)

/*
occurrenceOf turns an event into the fan-out it needs, or reports false
for an event no endpoint can subscribe to.
*/
func occurrenceOf(e any, now time.Time) (jobs.FanOutWebhookEventArgs, bool) {
	args, ok := namedOccurrence(e)
	if !ok {
		args, ok = contactOccurrence(e, now)
	}

	if !ok {
		return args, false
	}

	if _, listed := appevents.FindWebhookEventType(args.Type); !listed {
		return args, false
	}

	if extra := extrasOf(e); len(extra) > 0 {
		args.Data, _ = json.Marshal(extra)
	}

	return args, true
}

/*
namedOccurrence reads an event that carries the common header: a
conversation, message, tag or erased contact event.
*/
func namedOccurrence(e any) (jobs.FanOutWebhookEventArgs, bool) {
	named, ok := e.(appevents.Named)
	if !ok {
		return jobs.FanOutWebhookEventArgs{}, false
	}

	h := named.Header()
	args := jobs.FanOutWebhookEventArgs{
		WorkspaceID: h.WorkspaceID, EventID: h.ID, Type: named.EventName(), OccurredAt: h.OccurredAt,
		ActorType: h.Actor.Type, ActorID: actorID(h.Actor.ID),
	}

	switch ev := e.(type) {
	case appevents.MessageEvent:
		args.Subject, args.SubjectID = subjectMessage, ev.Message()
	case appevents.ConversationEvent:
		args.Subject, args.SubjectID = subjectConversation, ev.Conversation()
	case appevents.TagCreated:
		args.Subject, args.SubjectID = subjectTag, ev.TagID
	case appevents.TagUpdated:
		args.Subject, args.SubjectID = subjectTag, ev.TagID
	case appevents.TagDeleted:
		args.Subject, args.SubjectID = subjectTag, ev.TagID
	case appevents.ContactErased:
		args.Subject, args.SubjectID = subjectContact, ev.ContactID
	case appevents.ContactMerged:
		args.Subject, args.SubjectID = subjectContact, ev.ContactID
	default:
		return engagementOccurrence(e, args)
	}

	return args, true
}

/*
contactOccurrence reads the contact events, which carry no header.
*/
func contactOccurrence(e any, now time.Time) (jobs.FanOutWebhookEventArgs, bool) {
	args := jobs.FanOutWebhookEventArgs{EventID: ids.New(), OccurredAt: now, ActorType: "system"}

	switch ev := e.(type) {
	case appevents.ContactCreated:
		args.WorkspaceID, args.Type, args.SubjectID = ev.WorkspaceID, "contact.created", ev.ContactID
		args.ActorType, args.ActorID = memberActor(ev.ActorID)
	case appevents.ContactUpdated:
		args.WorkspaceID, args.Type, args.SubjectID = ev.WorkspaceID, "contact.updated", ev.ContactID
		args.ActorType, args.ActorID = memberActor(ev.ActorID)
	case appevents.ContactEventTracked:
		args.WorkspaceID, args.Type, args.SubjectID = ev.WorkspaceID, ev.EventName(), ev.ContactID
		args.ActorType = "integration"
		args.Subject = subjectContactEvent

		return args, true
	default:
		return args, false
	}

	args.Subject = subjectContact

	return args, true
}

/*
memberActor names a member as the actor, or the system when there is
none.
*/
func memberActor(id ids.UUID) (string, *ids.UUID) {
	if id == ids.Nil() {
		return "system", nil
	}

	return "member", &id
}

/*
actorID returns the actor's id, nil for the system.
*/
func actorID(id ids.UUID) *ids.UUID {
	if id == ids.Nil() {
		return nil
	}

	return &id
}

/*
extrasOf returns the facts of the event its payload's data carries
beside the object.
*/
func extrasOf(e any) map[string]any {
	if extra, ok := conversationExtras(e); ok {
		return extra
	}

	return otherExtras(e)
}

/*
conversationExtras returns what a change to a conversation changed.
*/
func conversationExtras(e any) (map[string]any, bool) {
	switch ev := e.(type) {
	case appevents.ConversationAssigned:
		return assignmentChanges(ev), true
	case appevents.ConversationClosed:
		return reason(ev.Reason), true
	case appevents.ConversationReopened:
		return reason(ev.Reason), true
	case appevents.ConversationUnsnoozed:
		return reason(ev.Reason), true
	case appevents.ConversationSnoozed:
		return map[string]any{"snoozed_until": ev.Until}, true
	case appevents.ConversationPriorityChanged:
		return changes("priority", ev.From, ev.To), true
	case appevents.ConversationUpdated:
		return updateChanges(ev.Changes), true
	case appevents.ConversationTagged:
		return map[string]any{"tag_ids": publicIDs(models.PrefixTag, ev.TagIDs)}, true
	case appevents.ConversationUntagged:
		return map[string]any{"tag_ids": publicIDs(models.PrefixTag, ev.TagIDs)}, true
	}

	return nil, false
}

/*
otherExtras returns the facts of the remaining events.
*/
func otherExtras(e any) map[string]any {
	switch ev := e.(type) {
	case appevents.ConversationMerged:
		return map[string]any{"merged_id": models.PublicID(models.PrefixConversation, ev.SourceID)}
	case appevents.ConversationParticipantsChanged:
		return map[string]any{
			"added": publicIDs(models.PrefixContact, ev.Added), "removed": publicIDs(models.PrefixContact, ev.Removed),
		}
	case appevents.ConversationRated:
		return map[string]any{"rating": map[string]any{
			"score": ev.Score, "comment": nullable(ev.Comment), "channel": ev.Channel,
			"member_id": publicID(models.PrefixMember, ev.MemberID),
		}}
	case appevents.AiHandoffRequested:
		return map[string]any{"handoff": map[string]any{"reason": ev.Reason, "summary": ev.Summary}}
	case appevents.MessageDeliveryUpdated:
		return map[string]any{"delivery_status": ev.Status, "delivery_error": nullable(ev.Reason)}
	case appevents.ContactUpdated:
		return map[string]any{"changes": ev.Changes, "changed_attributes": ev.ChangedAttributeKeys}
	case appevents.ContactEventTracked:
		return map[string]any{"name": ev.Name, "ref": nullable(ev.Ref)}
	}

	return engagementExtras(e)
}

/*
assignmentChanges says who a conversation was assigned from and to.
*/
func assignmentChanges(ev appevents.ConversationAssigned) map[string]any {
	out := map[string]any{}
	if !sameID(ev.FromMemberID, ev.ToMemberID) {
		out["assignee"] = map[string]any{
			"from": publicID(models.PrefixMember, ev.FromMemberID), "to": publicID(models.PrefixMember, ev.ToMemberID),
		}
	}

	if !sameID(ev.FromTeamID, ev.ToTeamID) {
		out["team"] = map[string]any{
			"from": publicID(models.PrefixTeam, ev.FromTeamID), "to": publicID(models.PrefixTeam, ev.ToTeamID),
		}
	}

	extra := map[string]any{"changes": out}
	if ev.Reason != "" {
		extra["reason"] = ev.Reason
	}

	return extra
}

/*
updateChanges writes a conversation's changed fields as from and to.
*/
func updateChanges(c map[string][2]any) map[string]any {
	out := make(map[string]any, len(c))
	for field, pair := range c {
		out[field] = map[string]any{"from": pair[0], "to": pair[1]}
	}

	return map[string]any{"changes": out}
}

/*
changes writes one field's change.
*/
func changes(field string, from, to any) map[string]any {
	return map[string]any{"changes": map[string]any{field: map[string]any{"from": from, "to": to}}}
}

/*
reason carries why a change happened, nothing when a person made it.
*/
func reason(r string) map[string]any {
	return map[string]any{"reason": nullable(r)}
}

/*
nullable writes an empty string as null.
*/
func nullable(s string) any {
	if s == "" {
		return nil
	}

	return s
}

/*
sameID reports whether two optional ids are equal.
*/
func sameID(a, b *ids.UUID) bool {
	if a == nil || b == nil {
		return a == b
	}

	return *a == *b
}

/*
publicID writes an optional id in its public form.
*/
func publicID(prefix models.Prefix, id *ids.UUID) any {
	if id == nil {
		return nil
	}

	return models.PublicID(prefix, *id)
}

/*
publicIDs writes ids in their public form.
*/
func publicIDs(prefix models.Prefix, list []ids.UUID) []string {
	out := make([]string, 0, len(list))
	for _, id := range list {
		out = append(out, models.PublicID(prefix, id))
	}

	return out
}
