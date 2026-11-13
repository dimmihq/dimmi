/*
|--------------------------------------------------------------------------
| Post Widget Message
|--------------------------------------------------------------------------
|
| A visitor's message enters the inbox the way every channel's does, through
| the inbox's inbound path, so a conversation the messenger starts behaves
| like any other: the same events, the same reopening rules, the same spam
| handling. The messenger adds what only it knows: which widget and session
| it came from, the page the visitor was on, whether they were signed in,
| and the id their browser gave the message, which makes a retried send land
| once.
|
| A visitor's write to a conversation closed long ago, or to imported
| history, starts a new conversation that names the old one, and the answer
| says so, so the messenger switches to the new thread.
|
*/

package widget

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/support/str"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Limits on what a visitor sends in one message. The request itself allows
at most ten files.
*/
const (
	maxMessageBytes = 64 << 10
	botBlockScore   = 70
)

/*
Receiver takes a message into the inbox, as every channel does.
*/
type Receiver interface {
	/*
		ReceiveInboundMessage stores the message in its conversation.
	*/
	ReceiveInboundMessage(ctx context.Context, msg contracts.InboundMessage) (contracts.InboundResult, error)
}

/*
OnlineChecker says whether someone on the team is there to answer.
*/
type OnlineChecker interface {
	/*
		Online reports whether the widget's team is there now.
	*/
	Online(ctx context.Context, r Resolved) (bool, error)
}

/*
MessageInput is a visitor's message: the session it comes from, the
id the browser gave it, its text and uploads, and the page it was sent on.
Conversation names the conversation it continues; nil starts one.
*/
type MessageInput struct {
	Auth          Authenticated
	Conversation  *ids.UUID
	ClientID      string
	Body          string
	AttachmentIDs []ids.UUID
	Page          *Page
	Context       map[string]any
	Browser       string
	OS            string
	Choice        string
}

/*
PostedMessage is where a visitor's message landed. Duplicate says the
browser had sent it before; RedirectedFrom names the conversation it was
sent to when a new one took it instead.
*/
type PostedMessage struct {
	Conversation   models.Conversation
	Message        models.Message
	Created        bool
	Duplicate      bool
	Reopened       bool
	RedirectedFrom *ids.UUID
}

/*
PostWidgetMessage takes visitors' messages into the inbox. Receiver is the
inbox's inbound path and Team says whether anyone is there to answer.
*/
type PostWidgetMessage struct {
	DB       *database.DB
	Clock    clock.Clock
	Receiver Receiver
	Team     OnlineChecker
	Surfaces *Surfaces
	Limits   *VisitorLimits
}

/*
Handle posts the message and returns where it landed.
*/
func (a *PostWidgetMessage) Handle(ctx context.Context, in MessageInput) (PostedMessage, error) {
	body, err := checkMessage(in)
	if err != nil {
		return PostedMessage{}, err
	}

	in.Body = body

	if err := a.Limits.Message(ctx, in.Auth); err != nil {
		return PostedMessage{}, err
	}

	var previous *models.FindWidgetConversationRow

	if in.Conversation != nil {
		found, err := a.find(ctx, in.Auth, *in.Conversation)
		if err != nil {
			return PostedMessage{}, err
		}

		previous = &found
	} else if err := a.mayStart(ctx, in.Auth); err != nil {
		return PostedMessage{}, err
	}

	result, err := a.Receiver.ReceiveInboundMessage(ctx, a.inbound(in, previous))
	if err != nil {
		return PostedMessage{}, err
	}

	return a.landed(ctx, in, previous, result)
}

/*
find returns the visitor's conversation, by the same rule as their list.
*/
func (a *PostWidgetMessage) find(
	ctx context.Context, auth Authenticated, id ids.UUID,
) (models.FindWidgetConversationRow, error) {
	var out models.FindWidgetConversationRow

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = models.New(tx).FindWidgetConversation(ctx, scopeOf(auth, id))

		return err
	})
	if errors.Is(err, database.ErrNotFound) {
		return out, exceptions.ErrWidgetConversationNotFound
	}

	if err != nil {
		return out, fmt.Errorf("find widget conversation: %w", err)
	}

	return out, nil
}

/*
requireEmail refuses a lead without an email when the widget asks for one
while the team is away, decided at the moment of sending.
*/
func (a *PostWidgetMessage) requireEmail(ctx context.Context, auth Authenticated) error {
	if !auth.Resolved.Widget.RequireEmailWhenOffline {
		return nil
	}

	var contact models.Contact

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		contact, err = models.New(tx).FindContact(ctx, models.FindContactParams{
			WorkspaceID: auth.Session.WorkspaceID, ID: auth.Session.ContactID,
		})

		return err
	})
	if err != nil {
		return fmt.Errorf("find widget contact: %w", err)
	}

	if contact.Email != nil {
		return nil
	}

	online, err := a.Team.Online(ctx, auth.Resolved)
	if err != nil {
		return err
	}

	if online {
		return nil
	}

	return exceptions.ErrEmailRequired
}

/*
inbound returns the message as the inbox takes it.
*/
func (a *PostWidgetMessage) inbound(
	in MessageInput, previous *models.FindWidgetConversationRow,
) contracts.InboundMessage {
	s, now := in.Auth.Session, a.Clock.Now()
	contact := s.ContactID

	msg := contracts.InboundMessage{
		ConnectionID: in.Auth.Resolved.Widget.ChannelConnectionID,
		DedupeKey:    "widget:" + s.ID.String() + ":" + strings.ToLower(in.ClientID),
		Sender:       contracts.InboundSender{ContactID: &contact, Kind: enums.ContactKindLead},
		Body:         in.Body, SentAt: now, IsTest: s.Test,
		Source: source(in, now),
		Metadata: map[string]any{
			"session_id": models.PublicID(models.PrefixWidgetSession, s.ID), "identity_verified": verified(s, now),
			"email_verified": EmailProven(s, now),
		},
	}

	if in.Choice != "" {
		msg.Metadata["answer"] = map[string]any{"path_id": in.Choice}
	}

	if s.BotScore >= botBlockScore {
		msg.SpamVerdict = enums.SpamVerdictSpam
	}

	if previous != nil && previous.ChannelType != "import" {
		id := previous.Conversation.ID
		msg.ConversationID = &id
	}

	msg.AfterPersist = func(ctx context.Context, tx *database.Tx, conv models.Conversation, m models.Message) error {
		return a.afterPersist(ctx, tx, in, persisted{conv: conv, message: m, previous: previous})
	}

	return msg
}

/*
persisted is a stored message with its conversation and the conversation the
visitor wrote to, when they named one.
*/
type persisted struct {
	conv     models.Conversation
	message  models.Message
	previous *models.FindWidgetConversationRow
}

/*
afterPersist records, inside the inbox's transaction, the browser's id for
the message, its uploads, and for a new conversation whether it is a test
and which one it continues.
*/
func (a *PostWidgetMessage) afterPersist(ctx context.Context, tx *database.Tx, in MessageInput, p persisted) error {
	q, now, ws := models.New(tx), a.Clock.Now(), p.conv.WorkspaceID

	client, err := json.Marshal(map[string]any{"client_id": in.ClientID, "page_url": pageURL(in.Page)})
	if err != nil {
		return fmt.Errorf("encode message client: %w", err)
	}

	if err := q.StampWidgetMessage(ctx, models.StampWidgetMessageParams{
		Client: client, Now: now, WorkspaceID: ws, ID: p.message.ID,
	}); err != nil {
		return fmt.Errorf("stamp widget message: %w", err)
	}

	if err := a.attach(ctx, q, in, p.message); err != nil {
		return err
	}

	if p.conv.MessageCount > 1 && (p.previous == nil || p.previous.Conversation.ID == p.conv.ID) {
		return nil
	}

	var before *ids.UUID
	if p.previous != nil && p.previous.Conversation.ID != p.conv.ID {
		before = &p.previous.Conversation.ID
	}

	err = q.MarkWidgetConversationStart(ctx, models.MarkWidgetConversationStartParams{
		IsTest: in.Auth.Session.Test, PreviousConversationID: before, Now: now, WorkspaceID: ws, ID: p.conv.ID,
	})
	if err != nil {
		return fmt.Errorf("mark widget conversation: %w", err)
	}

	return nil
}

/*
attach links the session's finished uploads to the message, refusing the
message when any is missing, unfinished or someone else's.
*/
func (a *PostWidgetMessage) attach(ctx context.Context, q *models.Queries, in MessageInput, m models.Message) error {
	if len(in.AttachmentIDs) == 0 {
		return nil
	}

	s := in.Auth.Session

	uploads, err := q.ListWidgetUploads(ctx, models.ListWidgetUploadsParams{
		WorkspaceID: s.WorkspaceID, Ids: in.AttachmentIDs, SessionID: &s.ID,
	})
	if err != nil {
		return fmt.Errorf("list widget uploads: %w", err)
	}

	if len(uploads) != len(in.AttachmentIDs) {
		return exceptions.ErrAttachmentNotUploaded
	}

	linked, err := q.LinkAttachments(ctx, models.LinkAttachmentsParams{
		MessageID: &m.ID, Now: a.Clock.Now(), WorkspaceID: s.WorkspaceID, Ids: in.AttachmentIDs,
	})
	if err != nil {
		return fmt.Errorf("link widget uploads: %w", err)
	}

	if int(linked) != len(in.AttachmentIDs) {
		return exceptions.ErrAttachmentNotUploaded
	}

	return nil
}

/*
landed reads back where the message went.
*/
func (a *PostWidgetMessage) landed(
	ctx context.Context, in MessageInput, previous *models.FindWidgetConversationRow, r contracts.InboundResult,
) (PostedMessage, error) {
	out := PostedMessage{Created: r.ConversationCreated, Duplicate: r.Duplicate, Reopened: r.Reopened}

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		ws := in.Auth.Session.WorkspaceID

		var err error

		out.Conversation, err = q.FindConversation(ctx, models.FindConversationParams{
			WorkspaceID: ws, ID: r.ConversationID,
		})
		if err != nil {
			return fmt.Errorf("find posted conversation: %w", err)
		}

		out.Message, err = q.FindMessage(ctx, models.FindMessageParams{
			WorkspaceID: ws, ConversationID: r.ConversationID, ID: r.MessageID,
		})
		if err != nil {
			return fmt.Errorf("find posted message: %w", err)
		}

		return nil
	})

	if previous != nil && out.Conversation.ID != previous.Conversation.ID {
		out.RedirectedFrom = &previous.Conversation.ID
	}

	return out, err
}

/*
checkMessage returns the message's text cleaned, or why it is refused.
*/
func checkMessage(in MessageInput) (string, error) {
	if _, err := ids.Parse(in.ClientID); err != nil {
		return "", messageField("client_id", "Use a UUID.")
	}

	body := str.CleanText(in.Body)
	if body == "" && len(in.AttachmentIDs) == 0 {
		return "", messageField("body", "Write a message or attach a file.")
	}

	if len(body) > maxMessageBytes {
		return "", messageField("body", "Keep a message under 64 KB.")
	}

	return body, nil
}

/*
messageField returns the refusal of one field of a message.
*/
func messageField(field, message string) error {
	return contactField(field, message)
}

/*
scopeOf returns the lookup of one of the session's conversations on its
widget.
*/
func scopeOf(auth Authenticated, id ids.UUID) models.FindWidgetConversationParams {
	return models.FindWidgetConversationParams{
		WorkspaceID: auth.Session.WorkspaceID, ID: id, ContactID: auth.Session.ContactID,
		WidgetID: models.PublicID(models.PrefixWidget, auth.Resolved.Widget.ID), InboxID: auth.Resolved.InboxID,
		IncludeImported: auth.Session.Kind == string(enums.WidgetSessionIdentified),
	}
}

/*
verified reports whether the session proves who the visitor is right now:
signed in with a verified identity token that has not expired. A test
session never does.
*/
func verified(s models.WidgetSession, now time.Time) bool {
	return s.Kind == string(enums.WidgetSessionIdentified) && !s.Test && s.VerifiedAt != nil && now.Before(s.ExpiresAt)
}

/*
pageURL returns the cleaned address of the page, "" when there is none.
*/
func pageURL(p *Page) string {
	if p == nil {
		return ""
	}

	return CleanPageURL(p.URL)
}

/*
source returns what a new conversation records about where it started.
*/
func source(in MessageInput, now time.Time) map[string]any {
	s := in.Auth.Session
	out := map[string]any{
		"type": "widget", "widget_id": models.PublicID(models.PrefixWidget, in.Auth.Resolved.Widget.ID),
		"session_id": models.PublicID(models.PrefixWidgetSession, s.ID),
	}

	page := currentPage(s, in.Page, now)
	if page != nil {
		out["page_url"], out["page_title"], out["referrer"] = page.URL, page.Title, page.Referrer
	}

	var recent []Page
	if json.Unmarshal(s.RecentPages, &recent) == nil && len(recent) > 0 {
		out["recent_pages"] = recent
	}

	if merged := cleanContext(s.Context, in.Context); string(merged) != "{}" {
		out["context"] = merged
	}

	for key, value := range map[string]*string{
		"user_agent": s.UserAgent, "locale": s.Locale, "timezone": s.Timezone, "country": s.Country,
		"browser": optionalText(in.Browser), "os": optionalText(in.OS),
	} {
		if value != nil {
			out[key] = *value
		}
	}

	return out
}

/*
currentPage returns the page the message was sent on, or else the last one
the session saw.
*/
func currentPage(s models.WidgetSession, sent *Page, now time.Time) *Page {
	if sent != nil {
		if page, ok := cleanPage(*sent, now); ok {
			return &page
		}
	}

	var stored Page
	if json.Unmarshal(s.CurrentPage, &stored) != nil || stored.URL == "" {
		return nil
	}

	return &stored
}

/*
mayStart refuses a new conversation the messenger's start rule does not
allow this visitor, then one the widget wants an email for first.
*/
func (a *PostWidgetMessage) mayStart(ctx context.Context, auth Authenticated) error {
	if err := a.Limits.Conversation(ctx, auth); err != nil {
		return err
	}

	if a.Surfaces != nil {
		if err := a.Surfaces.MayStart(ctx, auth.Resolved.Widget, auth.Session); err != nil {
			return err
		}
	}

	return a.requireEmail(ctx, auth)
}
