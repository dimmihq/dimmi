/*
|--------------------------------------------------------------------------
| Create Widget
|--------------------------------------------------------------------------
|
| A widget and its channel connection are one thing: both are written in
| one transaction, so a widget can always reach its inbox. The count happens
| under a lock on the workspace, so two people creating at the limit cannot
| both get through. A limit that dropped never removes what exists; it only
| stops new ones.
|
*/

package widget

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/support/str"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/entitlements"
)

/*
DefaultName is what a widget is called until someone names it.
*/
const DefaultName = "Website"

/*
maxWidgetName is the longest a widget's name may be.
*/
const maxWidgetName = 60

/*
CreateWidgetInput is who adds a widget, its name, empty for the default, and
its inbox, nil for the workspace's default inbox.
*/
type CreateWidgetInput struct {
	Membership authz.Membership
	Name       string
	InboxID    *ids.UUID
}

/*
CreateWidget adds a widget with its channel connection.

Rules:
  - The caller needs widgets:manage.
  - The workspace's widgets stay within widget.max_per_workspace.

Events: [appevents.WidgetCreated].

Errors: [exceptions.ErrForbidden], [exceptions.ErrEntitlementLimitReached],
[exceptions.ErrInboxNotFound], or a wrapped database error.
*/
type CreateWidget struct {
	DB           *database.DB
	Events       *events.Dispatcher
	Clock        clock.Clock
	Entitlements contracts.Entitlements
}

/*
Handle adds the widget and returns it.
*/
func (a *CreateWidget) Handle(ctx context.Context, in CreateWidgetInput) (models.WidgetSetting, error) {
	if !in.Membership.Can("widgets:manage") {
		return models.WidgetSetting{}, exceptions.ErrForbidden
	}

	name := str.Clean(in.Name)
	if name == "" {
		name = DefaultName
	}

	if len([]rune(name)) > maxWidgetName {
		return models.WidgetSetting{}, exceptions.WidgetField("name", "Use at most 60 characters.")
	}

	var out models.WidgetSetting

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, ws := models.New(tx), in.Membership.Workspace.ID

		if err := a.checkLimit(ctx, q, ws); err != nil {
			return err
		}

		inbox, err := findInbox(ctx, q, ws, in.InboxID)
		if err != nil {
			return err
		}

		out, err = insertWidget(ctx, q, inbox, name, a.Clock)
		if err != nil {
			return err
		}

		return a.Events.Dispatch(ctx, tx, appevents.WidgetCreated{
			WorkspaceID: ws, WidgetID: out.ID, ActorID: in.Membership.Member.ID,
		})
	})

	return out, err
}

/*
checkLimit counts the live widgets under a lock on the workspace.
*/
func (a *CreateWidget) checkLimit(ctx context.Context, q *models.Queries, ws ids.UUID) error {
	if _, err := q.LockWorkspace(ctx, ws); err != nil {
		return fmt.Errorf("lock workspace: %w", err)
	}

	limit, err := a.Entitlements.Limit(ctx, ws, entitlements.KeyWidgetMaxPerWorkspace)
	if err != nil {
		return err
	}

	used, err := q.CountWidgetSettings(ctx, ws)
	if err != nil {
		return fmt.Errorf("count widgets: %w", err)
	}

	if limit != entitlements.Unlimited && used >= limit {
		return exceptions.ErrEntitlementLimitReached.
			With("key", entitlements.KeyWidgetMaxPerWorkspace).With("limit", limit).With("used", used)
	}

	return nil
}

/*
findInbox returns the named live inbox, or the workspace's default.
*/
func findInbox(ctx context.Context, q *models.Queries, ws ids.UUID, id *ids.UUID) (models.Inbox, error) {
	if id == nil {
		inbox, err := q.FindDefaultInbox(ctx, ws)
		if err != nil {
			return inbox, fmt.Errorf("find default inbox: %w", err)
		}

		return inbox, nil
	}

	inbox, err := q.FindInbox(ctx, models.FindInboxParams{WorkspaceID: ws, ID: *id})
	if errors.Is(err, database.ErrNotFound) || (err == nil && inbox.DeletedAt != nil) {
		return inbox, exceptions.ErrInboxNotFound
	}

	if err != nil {
		return inbox, fmt.Errorf("find inbox: %w", err)
	}

	return inbox, nil
}

/*
insertWidget writes the channel connection and the widget on it.
*/
func insertWidget(
	ctx context.Context, q *models.Queries, inbox models.Inbox, name string, clk clock.Clock,
) (models.WidgetSetting, error) {
	now := clk.Now()

	conn, err := q.CreateChannelConnection(ctx, models.CreateChannelConnectionParams{
		WorkspaceID: inbox.WorkspaceID, InboxID: inbox.ID, Type: string(enums.ChannelTypeWidget), Name: name,
		Status: "active", Config: json.RawMessage(`{}`), Now: now,
	})
	if err != nil {
		return models.WidgetSetting{}, fmt.Errorf("create widget channel: %w", err)
	}

	widget, err := q.CreateWidgetSettings(ctx, models.CreateWidgetSettingsParams{
		WorkspaceID: inbox.WorkspaceID, ChannelConnectionID: conn.ID, Name: name, Now: now,
	})
	if err != nil {
		return widget, fmt.Errorf("create widget: %w", err)
	}

	return widget, nil
}
