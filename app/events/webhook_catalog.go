/*
|--------------------------------------------------------------------------
| Webhook Catalog
|--------------------------------------------------------------------------
|
| The events a developer can subscribe an endpoint to, each with the
| kind of object its payload carries and the area it belongs to. Only
| events listed here ever leave the product; everything else stays
| internal. An endpoint subscribes by exact name, by one area such as
| conversation.*, or to everything with *. Wildcards include events added
| later, except the ones marked named-only, which are too frequent to
| arrive without being asked for by name.
|
*/

package events

import (
	"slices"
	"strings"
)

/*
WebhookEventType is one subscribable event. Type is its dotted name and
Object the kind of resource its payload carries. Group is the area the
dashboard lists it under. NotesOptIn marks events that can be about an
internal note, delivered only to endpoints that include notes.
NamedOnly keeps it out of wildcard subscriptions.
*/
type WebhookEventType struct {
	Type        string `json:"type"`
	Object      string `json:"object"`
	Group       string `json:"group"`
	Description string `json:"description"`
	NotesOptIn  bool   `json:"notes_opt_in"`
	NamedOnly   bool   `json:"named_only"`
}

/*
WebhookTestType is the event a test send carries when no other type is
asked for. It is never dispatched and cannot be subscribed to.
*/
const WebhookTestType = "webhook.test"

/*
WebhookWildcard subscribes an endpoint to every event.
*/
const WebhookWildcard = "*"

/*
WebhookCatalog returns every subscribable event, grouped by area.
*/
func WebhookCatalog() []WebhookEventType {
	return slices.Concat(conversationWebhooks(), messageWebhooks(), contactWebhooks(), otherWebhooks())
}

/*
conversationWebhooks lists the events about conversations.
*/
func conversationWebhooks() []WebhookEventType {
	c := func(name, description string) WebhookEventType {
		return WebhookEventType{
			Type: "conversation." + name, Object: "conversation", Group: "Conversations",
			Description: description,
		}
	}

	return []WebhookEventType{
		c("created", "A conversation started."),
		c("updated", "A conversation's subject or inbox changed."),
		c("assigned", "A conversation's assignee or team changed."),
		c("closed", "A conversation was closed."),
		c("reopened", "A closed conversation opened again."),
		c("snoozed", "A conversation was snoozed."),
		c("unsnoozed", "A snooze ended."),
		c("priority_changed", "A conversation's priority changed."),
		c("tagged", "Tags were added to a conversation."),
		c("untagged", "Tags were removed from a conversation."),
		c("marked_spam", "A conversation was marked as spam."),
		c("unmarked_spam", "A conversation was taken out of spam."),
		c("merged", "Another conversation was merged into this one."),
		c("participants_changed", "People were added to or removed from a conversation."),
		c("deleted", "A conversation moved to the trash."),
	}
}

/*
messageWebhooks lists the events about messages, which can be about
internal notes.
*/
func messageWebhooks() []WebhookEventType {
	m := func(name, description string) WebhookEventType {
		return WebhookEventType{
			Type: "message." + name, Object: "message", Group: "Messages",
			Description: description, NotesOptIn: true,
		}
	}

	return []WebhookEventType{
		m("created", "A message or note was added to a conversation."),
		m("updated", "A note was edited."),
		m("deleted", "A message or note was deleted."),
		m("delivery_updated", "A sent message was delivered, read or failed."),
	}
}

/*
contactWebhooks lists the events about contacts.
*/
func contactWebhooks() []WebhookEventType {
	return []WebhookEventType{
		{Type: "contact.created", Object: "contact", Group: "Contacts", Description: "A contact was added."},
		{Type: "contact.updated", Object: "contact", Group: "Contacts", Description: "A contact's details changed."},
		{Type: "contact.deleted", Object: "contact", Group: "Contacts", Description: "A contact was erased."},
		{
			Type: "contact_event.tracked", Object: "contact_event", Group: "Contacts", NamedOnly: true,
			Description: "Your app tracked an event for a contact. Only sent when named.",
		},
	}
}

/*
otherWebhooks lists the events about tags and the AI agent.
*/
func otherWebhooks() []WebhookEventType {
	return []WebhookEventType{
		{Type: "tag.created", Object: "tag", Group: "Tags", Description: "A tag was added."},
		{Type: "tag.updated", Object: "tag", Group: "Tags", Description: "A tag was renamed or recolored."},
		{Type: "tag.deleted", Object: "tag", Group: "Tags", Description: "A tag was deleted."},
		{
			Type: "ai.handoff_requested", Object: "conversation", Group: "AI agent",
			Description: "The AI agent handed a conversation to the team.",
		},
	}
}

/*
FindWebhookEventType returns the catalog entry of a type.
*/
func FindWebhookEventType(name string) (WebhookEventType, bool) {
	i := slices.IndexFunc(WebhookCatalog(), func(t WebhookEventType) bool { return t.Type == name })
	if i < 0 {
		return WebhookEventType{}, false
	}

	return WebhookCatalog()[i], true
}

/*
ValidWebhookSubscription reports whether a subscription entry is *, an
area wildcard of a catalog area such as conversation.*, or a catalog
type.
*/
func ValidWebhookSubscription(entry string) bool {
	if entry == WebhookWildcard {
		return true
	}

	if area, ok := strings.CutSuffix(entry, ".*"); ok {
		return slices.ContainsFunc(WebhookCatalog(), func(t WebhookEventType) bool { return areaOf(t.Type) == area })
	}

	_, ok := FindWebhookEventType(entry)

	return ok
}

/*
WebhookSubscribed reports whether a subscription receives the event
type: by name, or by a wildcard unless the type is named-only.
*/
func WebhookSubscribed(subscription []string, eventType string) bool {
	entry, ok := FindWebhookEventType(eventType)
	if !ok {
		return false
	}

	if slices.Contains(subscription, eventType) {
		return true
	}

	if entry.NamedOnly {
		return false
	}

	return slices.Contains(subscription, WebhookWildcard) || slices.Contains(subscription, areaOf(eventType)+".*")
}

/*
areaOf returns the first segment of a dotted type.
*/
func areaOf(eventType string) string {
	area, _, _ := strings.Cut(eventType, ".")

	return area
}
