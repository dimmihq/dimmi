/*
|--------------------------------------------------------------------------
| Managing WhatsApp Numbers
|--------------------------------------------------------------------------
|
| A workspace's numbers once connected: listed and read, moved to another
| inbox, and disconnected, which stops Meta delivering here and drops the
| token while the conversations stay. A workspace's own Meta app is given
| the address its deliveries go to and the token Meta echoes there.
|
*/

package channels

import (
	"context"
	"strings"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
connectedNumbers returns the numbers the workspace connected.
*/
func (w *WhatsApp) connectedNumbers(ctx context.Context, ws ids.UUID) ([]string, error) {
	var out []string

	err := w.read(ctx, func(q *models.Queries) error {
		rows, err := q.ListWhatsAppConnections(ctx, ws)
		for _, row := range rows {
			out = append(out, WhatsAppFactsOf(row).PhoneNumberID)
		}

		return err
	})

	return out, err
}

/*
List returns the workspace's WhatsApp connections.

Errors: [exceptions.ErrForbidden].
*/
func (w *WhatsApp) List(ctx context.Context, m authz.Membership) ([]models.ChannelConnection, error) {
	if !m.Can("settings:read") && !m.Can("channels:manage") {
		return nil, exceptions.ErrForbidden
	}

	ctx = tenancy.With(ctx, m.Workspace.ID.String())

	var out []models.ChannelConnection

	err := w.read(ctx, func(q *models.Queries) error {
		var err error
		out, err = q.ListWhatsAppConnections(ctx, m.Workspace.ID)

		return err
	})

	return out, err
}

/*
Find returns the workspace's WhatsApp connection.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound].
*/
func (w *WhatsApp) Find(ctx context.Context, m authz.Membership, id ids.UUID) (models.ChannelConnection, error) {
	if !m.Can("settings:read") && !m.Can("channels:manage") {
		return models.ChannelConnection{}, exceptions.ErrForbidden
	}

	return w.connection(tenancy.With(ctx, m.Workspace.ID.String()), m.Workspace.ID, id)
}

/*
Update moves the connection's new conversations to another inbox
and renames it.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound].
*/
func (w *WhatsApp) Update(ctx context.Context, m authz.Membership, id ids.UUID, inbox *ids.UUID) (
	models.ChannelConnection, error,
) {
	if !m.Can("channels:manage") {
		return models.ChannelConnection{}, exceptions.ErrForbidden
	}

	ctx = tenancy.With(ctx, m.Workspace.ID.String())

	conn, err := w.connection(ctx, m.Workspace.ID, id)
	if err != nil || inbox == nil {
		return conn, err
	}

	err = w.read(ctx, func(q *models.Queries) error {
		target, err := inboxFor(ctx, q, m.Workspace.ID, inbox)
		if err != nil {
			return err
		}

		conn, err = q.MoveChannelConnection(ctx, models.MoveChannelConnectionParams{
			InboxID: target, Now: w.Clock.Now(), WorkspaceID: m.Workspace.ID, ID: id,
		})

		return err
	})

	return conn, err
}

/*
Disconnect retires the connection: Meta stops sending its deliveries
here, its token goes, and its conversations stay.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound].
*/
func (w *WhatsApp) Disconnect(ctx context.Context, m authz.Membership, id ids.UUID) error {
	if !m.Can("channels:manage") {
		return exceptions.ErrForbidden
	}

	ctx = tenancy.With(ctx, m.Workspace.ID.String())

	conn, err := w.connection(ctx, m.Workspace.ID, id)
	if err != nil {
		return err
	}

	facts := WhatsAppFactsOf(conn)
	if token, err := w.Token(ctx, conn); err == nil && facts.AccountID != "" {
		_ = w.graph(token).Delete(ctx, facts.AccountID+"/subscribed_apps")
	}

	return w.read(ctx, func(q *models.Queries) error {
		_, err := q.DisconnectChannelConnection(ctx, models.DisconnectChannelConnectionParams{
			Now: w.Clock.Now(), WorkspaceID: m.Workspace.ID, ID: id,
		})

		return err
	})
}

/*
WebhookAddress is where a workspace's own Meta app delivers for the
connection, and the token Meta must echo when it checks it; empty for a
number connected through the install's own app.
*/
func (w *WhatsApp) WebhookAddress(ctx context.Context, conn models.ChannelConnection) (string, string) {
	if WhatsAppFactsOf(conn).Mode != WhatsAppModeOwn {
		return "", ""
	}

	secrets, err := w.open(ctx, conn)
	if err != nil {
		return "", ""
	}

	id := models.PublicID(models.PrefixChannelConnection, conn.ID)

	return strings.TrimRight(w.AppURL, "/") + "/webhooks/whatsapp/" + id, secrets.VerifyToken
}
