/*
|--------------------------------------------------------------------------
| Member Alert
|--------------------------------------------------------------------------
|
| One thing a teammate is told about, written once and read by every
| channel: a row in the bell, a live update, an email and a push. Each
| recipient gets their own copy, so the channels and the preview follow
| their own settings. Activity on one conversation folds into a single
| unread row per trigger, so a chatty customer is one line in the bell.
| Email and push wait as the trigger says and ask, when their moment
| comes, whether the alert still matters; the bell never waits.
|
*/

package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"
	fnotify "github.com/dimmihq/dimmi/framework/notifications"
	"github.com/dimmihq/dimmi/framework/notifications/channels/broadcast"
	"github.com/dimmihq/dimmi/framework/notifications/channels/webpush"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/resources/views/mail/accounts"
)

/*
AlertType names member alerts among notifications.
*/
const AlertType = "member_alert"

/*
EventCreated is the live update a member's dashboard hears when an alert
arrives.
*/
const EventCreated = "notification.created"

/*
Alert is one teammate's copy of something they are told about: the
trigger, the workspace and what it is about, its title as a translation
key with arguments, so every channel writes it in the recipient's
language, the customer's words, where it links, who caused it and when,
and the channels it goes to for this recipient. Its checker decides at
send time whether it still matters and is never stored.
*/
type Alert struct {
	Trigger        string            `json:"trigger"`
	WorkspaceID    ids.UUID          `json:"workspace_id"`
	Workspace      string            `json:"workspace"`
	ConversationID *ids.UUID         `json:"conversation_id,omitempty"`
	Number         int64             `json:"number,omitempty"`
	MessageID      *ids.UUID         `json:"message_id,omitempty"`
	CallID         *ids.UUID         `json:"call_id,omitempty"`
	TitleKey       string            `json:"title_key"`
	TitleArgs      map[string]string `json:"title_args"`
	Body           string            `json:"body,omitempty"`
	Link           string            `json:"link"`
	ManageLink     string            `json:"manage_link"`
	Unsubscribe    string            `json:"unsubscribe,omitempty"`
	ActorName      string            `json:"actor_name,omitempty"`
	OccurredAt     time.Time         `json:"occurred_at"`
	Channels       []string          `json:"channels"`
	checker        *Checker
}

/*
NewAlert returns an empty alert that checks with the checker at send
time, for decoding queued deliveries into.
*/
func NewAlert(c *Checker) *Alert { return &Alert{checker: c} }

/*
Type names the notification.
*/
func (*Alert) Type() string { return AlertType }

/*
Via returns the channels the alert goes to for its recipient.
*/
func (a *Alert) Via(fnotify.Notifiable) []string { return a.Channels }

/*
TenantID is the alert's workspace.
*/
func (a *Alert) TenantID() string { return a.WorkspaceID.String() }

/*
GroupKey folds activity of one trigger on one conversation, or on one
action call, into a single unread row.
*/
func (a *Alert) GroupKey(fnotify.Delivery) string {
	switch {
	case a.CallID != nil:
		return a.Trigger + ":" + a.CallID.String()
	case a.ConversationID != nil:
		return a.Trigger + ":" + a.ConversationID.String()
	}
	return ""
}

/*
title writes the alert's title in the delivery's language.
*/
func (a *Alert) title(ctx context.Context) string {
	args := i18n.Args{}
	for k, v := range a.TitleArgs {
		args[k] = v
	}
	return i18n.T(ctx, "notifications."+a.TitleKey, args)
}

/*
data is what the bell and the live update carry.
*/
func (a *Alert) data(ctx context.Context) map[string]any {
	out := map[string]any{
		"trigger": a.Trigger, "title": a.title(ctx), "body": a.Body, "link": a.Link, "actor_name": a.ActorName,
	}
	if a.ConversationID != nil {
		out["conversation_id"] = a.ConversationID.String()
		out["conversation_number"] = a.Number
	}
	if a.CallID != nil {
		out["action_call_id"] = a.CallID.String()
	}
	return out
}

/*
ToDatabase is the bell's row.
*/
func (a *Alert) ToDatabase(ctx context.Context, _ fnotify.Delivery) (any, error) {
	return a.data(ctx), nil
}

/*
ToBroadcast is the live update the member's dashboard hears.
*/
func (a *Alert) ToBroadcast(ctx context.Context, _ fnotify.Delivery) (broadcast.Message, error) {
	return broadcast.Message{Event: EventCreated, Data: a.data(ctx)}, nil
}

/*
ToMail is the email.
*/
func (a *Alert) ToMail(ctx context.Context, _ fnotify.Delivery) (mail.Mailable, error) {
	return MemberAlertMail{
		Title: a.title(ctx), Body: a.Body, Workspace: a.Workspace, URL: a.Link, ManageURL: a.ManageLink,
		Unsubscribe: a.Unsubscribe,
	}, nil
}

/*
pushPayload is what the service worker shows: a title, the text, where
a click goes, and a tag that replaces an older push about the same
thing.
*/
type pushPayload struct {
	Title string `json:"title"`
	Body  string `json:"body,omitempty"`
	URL   string `json:"url"`
	Tag   string `json:"tag"`
}

/*
ToWebPush is the push. Things a teammate must act on wake the device
at once; the rest wait for it to wake on its own.
*/
func (a *Alert) ToWebPush(ctx context.Context, d fnotify.Delivery) (webpush.Message, error) {
	tag := hashing.SHA256Hex([]byte(a.GroupKey(d) + a.TitleKey))[:24]
	payload, err := json.Marshal(pushPayload{Title: a.title(ctx), Body: truncate(a.Body, 180), URL: a.Link, Tag: tag})
	if err != nil {
		return webpush.Message{}, fmt.Errorf("encode push: %w", err)
	}
	urgency := webpush.UrgencyNormal
	switch a.Trigger {
	case TriggerAssigned, TriggerMentioned, TriggerHandoff, TriggerApprovalRequired:
		urgency = webpush.UrgencyHigh
	}
	return webpush.Message{Payload: payload, TTL: 4 * time.Hour, Urgency: urgency, Topic: tag}, nil
}

/*
QueueOptions hold email, push and Slack until the trigger's wait is
over; Slack waits as long as push.
*/
func (a *Alert) QueueOptions(d fnotify.Delivery) []queue.Opt {
	t, ok := FindTrigger(a.Trigger)
	if !ok {
		return nil
	}
	switch d.Channel {
	case "mail":
		return []queue.Opt{queue.At(a.OccurredAt.Add(t.EmailDelay))}
	case "webpush", SlackChannelName:
		return []queue.Opt{queue.At(a.OccurredAt.Add(t.PushDelay))}
	}
	return nil
}

/*
ShouldSend asks, when a delivery's moment comes, whether the alert still
matters to its recipient on that channel.
*/
func (a *Alert) ShouldSend(ctx context.Context, d fnotify.Delivery) (bool, error) {
	if a.checker == nil {
		return true, nil
	}
	return a.checker.Allow(ctx, a, d)
}

/*
MemberAlertMail is a member alert as an email. Unsubscribe is where a
mail app posts to turn this kind of email off.
*/
type MemberAlertMail struct {
	Title       string `json:"title"`
	Body        string `json:"body"`
	Workspace   string `json:"workspace"`
	URL         string `json:"url"`
	ManageURL   string `json:"manage_url"`
	Unsubscribe string `json:"unsubscribe,omitempty"`
}

/*
Envelope leaves the subject to the content and offers the one-click
unsubscribe.
*/
func (m MemberAlertMail) Envelope() mail.Envelope { return unsubscribeEnvelope(m.Unsubscribe) }

/*
unsubscribeEnvelope carries the RFC 8058 headers for the link, none
without one.
*/
func unsubscribeEnvelope(link string) mail.Envelope {
	if link == "" {
		return mail.Envelope{}
	}
	return mail.Envelope{Headers: map[string]string{
		"List-Unsubscribe":      "<" + link + ">",
		"List-Unsubscribe-Post": "List-Unsubscribe=One-Click",
	}}
}

/*
Content renders the mail with the alert's title as its subject.
*/
func (m MemberAlertMail) Content(context.Context) (mail.Content, error) {
	c := mail.View(accounts.MemberNotification(m.Title, m.Body, m.Workspace, m.URL, m.ManageURL, m.Unsubscribe))
	c.Subject = m.Title
	return c, nil
}

/*
truncate shortens text to at most n characters, marking the cut.
*/
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
