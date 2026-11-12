/*
|--------------------------------------------------------------------------
| Show Widget Settings
|--------------------------------------------------------------------------
|
| Reads a workspace's widgets for its settings page, with what the page shows
| beside each one: its inbox, its look with defaults filled in, and its
| badge. Anyone who may see settings may see these, including the install
| snippet they lead to.
|
*/

package widget

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
SettingsRow is a widget with what its settings page shows beside it, its
own words by language among them.
*/
type SettingsRow struct {
	Widget        models.WidgetSetting
	Inbox         models.Inbox
	Appearance    Appearance
	Texts         map[string]any
	TextsByLocale map[string]map[string]string
	Branding      Branding
}

/*
ShowWidgetSettings reads widgets. Every method needs widgets:read, or
widgets:manage, since whoever changes a widget sees what they changed.
*/
type ShowWidgetSettings struct {
	DB       *database.DB
	Branding *ResolveBranding
}

/*
List returns the workspace's widgets, oldest first.
*/
func (a *ShowWidgetSettings) List(ctx context.Context, m authz.Membership) ([]SettingsRow, error) {
	if !canRead(m) {
		return nil, exceptions.ErrForbidden
	}

	var out []SettingsRow

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		rows, err := q.ListWidgetSettings(ctx, m.Workspace.ID)
		if err != nil {
			return fmt.Errorf("list widgets: %w", err)
		}

		out = make([]SettingsRow, 0, len(rows))

		for _, row := range rows {
			shaped, err := a.shape(ctx, q, m, row)
			if err != nil {
				return err
			}

			out = append(out, shaped)
		}

		return nil
	})

	return out, err
}

/*
Find returns one widget of the workspace.
*/
func (a *ShowWidgetSettings) Find(ctx context.Context, m authz.Membership, id ids.UUID) (SettingsRow, error) {
	if !canRead(m) {
		return SettingsRow{}, exceptions.ErrForbidden
	}

	var out SettingsRow

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		row, err := q.FindWidgetSettings(ctx, models.FindWidgetSettingsParams{WorkspaceID: m.Workspace.ID, ID: id})
		if errors.Is(err, database.ErrNotFound) {
			return exceptions.ErrNotFound
		}

		if err != nil {
			return fmt.Errorf("find widget: %w", err)
		}

		out, err = a.shape(ctx, q, m, row)

		return err
	})

	return out, err
}

/*
shape reads what the page shows beside the widget.
*/
func (a *ShowWidgetSettings) shape(
	ctx context.Context, q *models.Queries, m authz.Membership, w models.WidgetSetting,
) (SettingsRow, error) {
	out := SettingsRow{Widget: w, Texts: map[string]any{}}

	conn, err := q.FindChannelConnection(ctx, models.FindChannelConnectionParams{
		WorkspaceID: w.WorkspaceID, ID: w.ChannelConnectionID,
	})
	if err != nil {
		return out, fmt.Errorf("find widget channel: %w", err)
	}

	out.Inbox, err = q.FindInbox(ctx, models.FindInboxParams{WorkspaceID: w.WorkspaceID, ID: conn.InboxID})
	if err != nil {
		return out, fmt.Errorf("find widget inbox: %w", err)
	}

	out.Appearance, err = ResolveAppearance(w.Appearance)
	if err != nil {
		return out, err
	}

	if err := json.Unmarshal(w.Texts, &out.Texts); err != nil {
		return out, fmt.Errorf("read widget texts: %w", err)
	}

	out.TextsByLocale, err = TextsByLocale(w)
	if err != nil {
		return out, err
	}

	out.Branding, err = a.Branding.Handle(ctx, m.Workspace, w.ShowBranding)

	return out, err
}

/*
canRead reports whether the member may see the workspace's widgets.
*/
func canRead(m authz.Membership) bool {
	return m.Can("widgets:read") || m.Can("widgets:manage")
}
