/*
|--------------------------------------------------------------------------
| WhatsApp In A Conversation
|--------------------------------------------------------------------------
|
| A teammate answering on WhatsApp needs to know whether free text can
| still go out, until when, and which templates can go instead. A
| template a reply is sent as is checked here before the reply is
| written: approved, sendable from the inbox, every variable given. The
| reply's body is the template as the customer will read it, so the
| thread shows what was sent.
|
*/

package channels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/auth/authz"
	wadriver "github.com/dimmihq/dimmi/app/drivers/channel/whatsapp"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
WhatsAppConversation is where a conversation stands on WhatsApp: its
connection, until when free text may go out, zero when never, and the
connection's templates.
*/
type WhatsAppConversation struct {
	Connection models.ChannelConnection
	Contact    models.Contact
	ExpiresAt  time.Time
	Open       bool
	Templates  []WhatsAppTemplate
}

/*
Conversation returns where the conversation stands on WhatsApp.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound] for a
conversation that is not the workspace's or not on WhatsApp.
*/
func (w *WhatsApp) Conversation(ctx context.Context, m authz.Membership, conversationID ids.UUID) (
	WhatsAppConversation, error,
) {
	if !m.Can("conversations:read") {
		return WhatsAppConversation{}, exceptions.ErrForbidden
	}

	ctx = tenancy.With(ctx, m.Workspace.ID.String())

	out, err := w.whereIs(ctx, m.Workspace.ID, conversationID)
	if err != nil {
		return out, err
	}

	window, err := windowOf(ctx, w.DB, out.Connection, out.Contact.ID)
	if err != nil {
		return out, err
	}

	out.ExpiresAt, out.Open = window.ExpiresAt, window.Open(w.Clock.Now())
	out.Templates, err = w.templatesOf(ctx, out.Connection)

	return out, err
}

/*
whereIs finds the conversation's WhatsApp connection and customer.
*/
func (w *WhatsApp) whereIs(ctx context.Context, ws, conversationID ids.UUID) (WhatsAppConversation, error) {
	var out WhatsAppConversation

	err := w.read(ctx, func(q *models.Queries) error {
		c, err := q.FindConversation(ctx, models.FindConversationParams{WorkspaceID: ws, ID: conversationID})
		if err != nil {
			return err
		}

		out.Connection, err = q.FindChannelConnection(ctx, models.FindChannelConnectionParams{
			WorkspaceID: ws, ID: c.ChannelConnectionID,
		})
		if err != nil {
			return err
		}

		out.Contact, err = q.FindContact(ctx, models.FindContactParams{WorkspaceID: ws, ID: c.ContactID})

		return err
	})
	if errors.Is(err, database.ErrNotFound) || (err == nil && out.Connection.Type != wadriver.Type) {
		return out, exceptions.ErrNotFound
	}

	return out, err
}

/*
TemplateReply is a template ready to go as a reply: its body as the
customer reads it and the channel options that send it.
*/
type TemplateReply struct {
	Body    string
	Options json.RawMessage
}

/*
PrepareTemplate checks the template the teammate picked for the
conversation and returns the reply that sends it.

Errors: as [WhatsApp.Conversation], and
[exceptions.ErrWhatsAppTemplateUnavailable],
[exceptions.ErrWhatsAppTemplateValueMissing].
*/
func (w *WhatsApp) PrepareTemplate(
	ctx context.Context, m authz.Membership, conversationID ids.UUID, choice wadriver.Choice,
) (TemplateReply, error) {
	if !m.Can("conversations:write") {
		return TemplateReply{}, exceptions.ErrForbidden
	}

	ctx = tenancy.With(ctx, m.Workspace.ID.String())

	where, err := w.whereIs(ctx, m.Workspace.ID, conversationID)
	if err != nil {
		return TemplateReply{}, err
	}

	t, found, err := (&WhatsAppPlaces{DB: w.DB, Clock: w.Clock}).Template(ctx, where.Connection, choice.Name,
		choice.Language)
	if err != nil {
		return TemplateReply{}, err
	}

	if !found || !t.Sendable() {
		return TemplateReply{}, exceptions.ErrWhatsAppTemplateUnavailable
	}

	values := map[string]string{}
	for _, v := range t.Variables() {
		value := strings.TrimSpace(firstOf(choice.Values[v.Part+"."+v.Key], choice.Values[v.Key]))
		if value == "" {
			return TemplateReply{}, exceptions.ErrWhatsAppTemplateValueMissing.OnField("values." + v.Part + "." + v.Key)
		}

		values[v.Part+"."+v.Key] = value
	}

	choice.Values = values

	options, err := json.Marshal(map[string]any{wadriver.Type: wadriver.Options{Template: &choice}})
	if err != nil {
		return TemplateReply{}, fmt.Errorf("encode template options: %w", err)
	}

	return TemplateReply{Body: t.Render(values), Options: options}, nil
}
