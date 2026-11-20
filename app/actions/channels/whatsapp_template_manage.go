/*
|--------------------------------------------------------------------------
| Managing Templates
|--------------------------------------------------------------------------
|
| Teammates who manage channels write a number's templates in dimmi: a
| new one goes to Meta for review, an edited one goes back to review, and
| a deleted one is gone from Meta. Each is checked here first, so what
| Meta would refuse at once is fixed on its field; what Meta still refuses
| comes back in Meta's words. dimmi's copy of the templates is taken again
| straight after, so the list shows the template as Meta has it, waiting
| for review; Meta's verdict arrives on the webhook.
|
*/

package channels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/auth/authz"
	wadriver "github.com/dimmihq/dimmi/app/drivers/channel/whatsapp"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
CreateTemplate sends a new template to Meta for review and returns it as
dimmi now keeps it.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrWhatsAppTemplateInvalid],
[exceptions.ErrWhatsAppTemplateRefused],
[exceptions.ErrWhatsAppUnavailable].
*/
func (w *WhatsApp) CreateTemplate(ctx context.Context, m authz.Membership, id ids.UUID, in WhatsAppDraft) (
	WhatsAppTemplate, error,
) {
	d := in.Draft

	d.Buttons = groupButtons(d.Buttons)
	if err := checkNewDraft(d); err != nil {
		return WhatsAppTemplate{}, err
	}

	at, err := w.templateWriter(ctx, m, id)
	if err != nil {
		return WhatsAppTemplate{}, err
	}

	written, err := at.graph.CreateTemplate(ctx, at.account, d)
	if err != nil {
		return WhatsAppTemplate{}, w.templateRefused(err)
	}

	return w.keptTemplate(ctx, at.conn, written.ID, in.bound())
}

/*
UpdateTemplate sends a template's new content to Meta, which reviews it
again. Its name and language stay.

Errors: as [WhatsApp.CreateTemplate].
*/
func (w *WhatsApp) UpdateTemplate(ctx context.Context, m authz.Membership, ref WhatsAppTemplateRef,
	in WhatsAppDraft,
) (WhatsAppTemplate, error) {
	d := in.Draft

	at, err := w.templateWriter(ctx, m, ref.Channel)
	if err != nil {
		return WhatsAppTemplate{}, err
	}

	row, err := w.templateRow(ctx, at.conn, ref.Template)
	if err != nil {
		return WhatsAppTemplate{}, err
	}

	d.Name, d.Language, d.Buttons = row.Name, row.Language, groupButtons(d.Buttons)
	if err := checkDraft(d); err != nil {
		return WhatsAppTemplate{}, err
	}

	if d.Category == row.Category {
		d.Category = ""
	}

	if err := at.graph.EditTemplate(ctx, row.ExternalID, d); err != nil {
		return WhatsAppTemplate{}, w.templateRefused(err)
	}

	return w.keptTemplate(ctx, at.conn, row.ExternalID, in.bound())
}

/*
DeleteTemplate removes a template from Meta and from dimmi.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrWhatsAppTemplateRefused], [exceptions.ErrWhatsAppUnavailable].
*/
func (w *WhatsApp) DeleteTemplate(ctx context.Context, m authz.Membership, ref WhatsAppTemplateRef) error {
	at, err := w.templateWriter(ctx, m, ref.Channel)
	if err != nil {
		return err
	}

	conn := at.conn

	row, err := w.templateRow(ctx, conn, ref.Template)
	if err != nil {
		return err
	}

	if err := at.graph.DeleteTemplate(ctx, at.account, row.ExternalID, row.Name); err != nil {
		return w.templateRefused(err)
	}

	return w.read(ctx, func(q *models.Queries) error {
		return q.DeleteWhatsAppTemplate(ctx, models.DeleteWhatsAppTemplateParams{
			WorkspaceID: conn.WorkspaceID, ChannelConnectionID: conn.ID, ID: row.ID,
		})
	})
}

/*
WhatsAppTemplateRef names one template of a connection.
*/
type WhatsAppTemplateRef struct {
	Channel  ids.UUID
	Template ids.UUID
}

/*
templateAccess is a connection whose templates a teammate manages, a
Graph client with its token, and its WhatsApp account.
*/
type templateAccess struct {
	conn    models.ChannelConnection
	graph   *wadriver.Graph
	account string
}

/*
templateWriter returns what writing the connection's templates needs.
*/
func (w *WhatsApp) templateWriter(ctx context.Context, m authz.Membership, id ids.UUID) (templateAccess, error) {
	if !m.Can("channels:manage") {
		return templateAccess{}, exceptions.ErrForbidden
	}

	conn, err := w.connection(tenancy.With(ctx, m.Workspace.ID.String()), m.Workspace.ID, id)
	if err != nil {
		return templateAccess{}, err
	}

	account := WhatsAppFactsOf(conn).AccountID
	if account == "" {
		return templateAccess{}, exceptions.ErrNotFound
	}

	token, err := w.Token(ctx, conn)
	if err != nil {
		return templateAccess{}, err
	}

	return templateAccess{conn: conn, graph: w.graph(token), account: account}, nil
}

/*
templateRow returns the connection's template with the id.
*/
func (w *WhatsApp) templateRow(ctx context.Context, conn models.ChannelConnection, id ids.UUID) (
	models.WhatsappTemplate, error,
) {
	var row models.WhatsappTemplate

	err := w.read(ctx, func(q *models.Queries) error {
		var err error
		row, err = q.FindWhatsAppTemplateByID(ctx, models.FindWhatsAppTemplateByIDParams{
			WorkspaceID: conn.WorkspaceID, ChannelConnectionID: conn.ID, ID: id,
		})

		return err
	})
	if errors.Is(err, database.ErrNotFound) {
		return row, exceptions.ErrNotFound
	}

	if err != nil {
		return row, fmt.Errorf("find whatsapp template: %w", err)
	}

	return row, nil
}

/*
WhatsAppDraft is a template a teammate wrote, with the customer detail
each of its variables stands for, keyed like its samples: "body.1".
*/
type WhatsAppDraft struct {
	wadriver.Draft

	Bindings map[string]WhatsAppBinding
}

/*
WhatsAppBinding is the detail a variable is filled in with when the
template is picked, such as contact.first_name, and what it reads as for
a customer without it.
*/
type WhatsAppBinding struct {
	Key      string `json:"key"`
	Fallback string `json:"fallback,omitempty"`
}

var bindingKey = regexp.MustCompile(`^[a-z_]+(\.[a-z_]+)+$`)

/*
bound returns the bindings of variables the draft has, each with a key
dimmi can read.
*/
func (d WhatsAppDraft) bound() map[string]WhatsAppBinding {
	out := map[string]WhatsAppBinding{}

	for part, text := range map[string]string{"header": d.Header, "body": d.Body} {
		for _, name := range wadriver.Placeholders(text) {
			b, ok := d.Bindings[part+"."+name]
			if ok && len(b.Key) <= bindingKeyMax && len(b.Fallback) <= bindingKeyMax && bindingKey.MatchString(b.Key) {
				out[part+"."+name] = b
			}
		}
	}

	return out
}

/*
keptTemplate takes a new copy of the connection's templates, records
what the template's variables stand for, and returns it by Meta's id.
*/
func (w *WhatsApp) keptTemplate(
	ctx context.Context, conn models.ChannelConnection, externalID string, bindings map[string]WhatsAppBinding,
) (WhatsAppTemplate, error) {
	if err := w.SyncTemplates(ctx, conn); err != nil {
		return WhatsAppTemplate{}, err
	}

	raw, err := json.Marshal(bindings)
	if err != nil {
		return WhatsAppTemplate{}, fmt.Errorf("encode whatsapp template bindings: %w", err)
	}

	err = w.read(ctx, func(q *models.Queries) error {
		return q.SetWhatsAppTemplateBindings(ctx, models.SetWhatsAppTemplateBindingsParams{
			WorkspaceID: conn.WorkspaceID, ChannelConnectionID: conn.ID, ExternalID: externalID, Bindings: raw,
			Now: w.Clock.Now(),
		})
	})
	if err != nil {
		return WhatsAppTemplate{}, fmt.Errorf("keep whatsapp template bindings: %w", err)
	}

	rows, err := w.templateRows(ctx, conn)
	if err != nil {
		return WhatsAppTemplate{}, err
	}

	for _, row := range rows {
		if row.ExternalID == externalID {
			t := templateOf(row)

			return WhatsAppTemplate{Row: row, Template: t, Variables: t.Variables(), Preview: t.Render(nil)}, nil
		}
	}

	return WhatsAppTemplate{}, exceptions.ErrNotFound
}

/*
templateRefused turns Meta's refusal into the teammate's error, in Meta's
own words when it gave some.
*/
func (w *WhatsApp) templateRefused(err error) error {
	refused, ok := errors.AsType[*wadriver.GraphError](err)
	if !ok {
		return w.unavailable(err)
	}

	switch refused.Code {
	case 190, 10, 200:
		return exceptions.ErrWhatsAppTokenRejected
	case 100, 192:
		message := refused.UserMessage
		if message == "" {
			message = refused.Details
		}

		return fmt.Errorf("%w: %w", exceptions.WhatsAppTemplateRefused(message), err)
	default:
		return w.unavailable(err)
	}
}

/*
groupButtons puts quick replies before the other buttons, as Meta wants
them grouped, keeping each group's order.
*/
func groupButtons(buttons []wadriver.Action) []wadriver.Action {
	out := slices.Clone(buttons)
	slices.SortStableFunc(out, func(a, b wadriver.Action) int {
		return quickFirst(a) - quickFirst(b)
	})

	return out
}

func quickFirst(b wadriver.Action) int {
	if b.Type == wadriver.ButtonQuickReply {
		return 0
	}

	return 1
}
