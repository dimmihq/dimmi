/*
|--------------------------------------------------------------------------
| WhatsApp Templates
|--------------------------------------------------------------------------
|
| Templates are written and approved in Meta's WhatsApp Manager; dimmi
| keeps a copy of each, so the composer lists them at once. The copy is
| taken when a number connects, when a teammate asks, and whenever the
| composer finds it older than a few minutes. Templates Meta no longer
| lists go from the copy.
|
*/

package channels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
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
templateFreshness is how old the copy of a connection's templates may
be before the composer takes a new one.
*/
const templateFreshness = 10 * time.Minute

/*
SyncTemplates copies the connection's templates from Meta.
*/
func (w *WhatsApp) SyncTemplates(ctx context.Context, conn models.ChannelConnection) error {
	facts := WhatsAppFactsOf(conn)
	if facts.AccountID == "" {
		return nil
	}

	token, err := w.Token(ctx, conn)
	if err != nil {
		return err
	}

	templates, err := w.graph(token).ListTemplates(ctx, facts.AccountID)
	if err != nil {
		return w.unavailable(err)
	}

	started := w.Clock.Now()

	return w.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		for _, t := range templates {
			if err := upsertTemplate(ctx, q, templateCopy{conn: conn, template: t, at: started}); err != nil {
				return err
			}
		}

		return q.DeleteStaleWhatsAppTemplates(ctx, models.DeleteStaleWhatsAppTemplatesParams{
			WorkspaceID: conn.WorkspaceID, ChannelConnectionID: conn.ID, StartedAt: started,
		})
	})
}

/*
templateCopy is one of Meta's templates on its way onto a connection,
taken at the time.
*/
type templateCopy struct {
	conn     models.ChannelConnection
	template wadriver.Template
	at       time.Time
}

/*
upsertTemplate copies one template onto the connection.
*/
func upsertTemplate(ctx context.Context, q *models.Queries, c templateCopy) error {
	t := c.template

	category := strings.ToLower(t.Category)
	if category != "marketing" && category != "utility" && category != "authentication" {
		return nil
	}

	components, err := json.Marshal(storedTemplate{ParameterFormat: t.ParameterFormat, Components: t.Components})
	if err != nil {
		return fmt.Errorf("encode whatsapp template: %w", err)
	}

	err = q.UpsertWhatsAppTemplate(ctx, models.UpsertWhatsAppTemplateParams{
		WorkspaceID: c.conn.WorkspaceID, ChannelConnectionID: c.conn.ID, ExternalID: t.ID, Name: t.Name,
		Language: t.Language, Category: category, Status: strings.ToLower(t.Status), Components: components,
		ParameterCount: storedCount(len(t.Variables())), Sendable: t.Sendable(), RejectedReason: optional(rejection(t)),
		Now: c.at,
	})
	if err != nil {
		return fmt.Errorf("keep whatsapp template: %w", err)
	}

	return nil
}

/*
storedCount returns a count as the database's integer, capped at its largest
value.
*/
func storedCount(n int) int32 {
	if n > math.MaxInt32 {
		return math.MaxInt32
	}

	return int32(n) //nolint:gosec // bounded above
}

/*
rejection is why Meta rejected a template, empty when it did not.
*/
func rejection(t wadriver.Template) string {
	if t.RejectedReason == "" || strings.EqualFold(t.RejectedReason, "NONE") {
		return ""
	}

	return t.RejectedReason
}

/*
WhatsAppTemplate is a template as the inbox shows it: its row, Meta's
description, what it needs filled in, and its text as the customer
reads it.
*/
type WhatsAppTemplate struct {
	Row       models.WhatsappTemplate
	Template  wadriver.Template
	Variables []wadriver.Variable
	Preview   string
}

/*
Templates returns the connection's templates, taking a new copy from
Meta first when the copy is old. A copy Meta could not refresh is still
returned.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound].
*/
func (w *WhatsApp) Templates(ctx context.Context, m authz.Membership, id ids.UUID) ([]WhatsAppTemplate, error) {
	conn, err := w.templateConnection(ctx, m, id)
	if err != nil {
		return nil, err
	}

	return w.templatesOf(ctx, conn)
}

/*
RefreshTemplates takes a new copy of the connection's templates from
Meta and returns them.

Errors: as [WhatsApp.Templates], and [exceptions.ErrWhatsAppUnavailable].
*/
func (w *WhatsApp) RefreshTemplates(ctx context.Context, m authz.Membership, id ids.UUID) (
	[]WhatsAppTemplate, error,
) {
	conn, err := w.templateConnection(ctx, m, id)
	if err != nil {
		return nil, err
	}

	if err := w.SyncTemplates(ctx, conn); err != nil {
		return nil, err
	}

	return w.templatesOf(ctx, conn)
}

/*
templateConnection returns the workspace's connection whose templates a
teammate may read.
*/
func (w *WhatsApp) templateConnection(ctx context.Context, m authz.Membership, id ids.UUID) (
	models.ChannelConnection, error,
) {
	if !m.Can("conversations:read") && !m.Can("channels:manage") {
		return models.ChannelConnection{}, exceptions.ErrForbidden
	}

	return w.connection(tenancy.With(ctx, m.Workspace.ID.String()), m.Workspace.ID, id)
}

/*
templatesOf returns the connection's templates, taking a new copy first
when the one kept is old. A copy Meta could not refresh is still
returned.
*/
func (w *WhatsApp) templatesOf(ctx context.Context, conn models.ChannelConnection) ([]WhatsAppTemplate, error) {
	rows, err := w.templateRows(ctx, conn)
	if err != nil {
		return nil, err
	}

	if stale(rows, w.Clock.Now()) && w.SyncTemplates(ctx, conn) == nil {
		if rows, err = w.templateRows(ctx, conn); err != nil {
			return nil, err
		}
	}

	out := make([]WhatsAppTemplate, 0, len(rows))
	for _, row := range rows {
		t := templateOf(row)
		out = append(out, WhatsAppTemplate{Row: row, Template: t, Variables: t.Variables(), Preview: t.Render(nil)})
	}

	return out, nil
}

/*
templateRows reads the connection's copied templates.
*/
func (w *WhatsApp) templateRows(ctx context.Context, conn models.ChannelConnection) ([]models.WhatsappTemplate, error) {
	var rows []models.WhatsappTemplate

	err := w.read(ctx, func(q *models.Queries) error {
		var err error
		rows, err = q.ListWhatsAppTemplates(ctx, models.ListWhatsAppTemplatesParams{
			WorkspaceID: conn.WorkspaceID, ChannelConnectionID: conn.ID,
		})

		return err
	})
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return nil, fmt.Errorf("list whatsapp templates: %w", err)
	}

	return rows, nil
}

/*
stale reports whether the copy is old enough to take again, or empty.
*/
func stale(rows []models.WhatsappTemplate, now time.Time) bool {
	if len(rows) == 0 {
		return true
	}

	oldest := rows[0].SyncedAt
	for _, row := range rows {
		if row.SyncedAt.Before(oldest) {
			oldest = row.SyncedAt
		}
	}

	return now.Sub(oldest) > templateFreshness
}
