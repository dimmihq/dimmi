/*
|--------------------------------------------------------------------------
| Delete Widget
|--------------------------------------------------------------------------
|
| Removing a widget stops it at once: its config answers not found, its
| visitors' sessions end and their streams close. Conversations it started
| stay in the inbox with their history, and its connection stays as
| disconnected so they keep naming where they came from.
|
*/

package widget

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
DeleteWidgetInput is who deletes which widget.
*/
type DeleteWidgetInput struct {
	Membership authz.Membership
	WidgetID   ids.UUID
}

/*
SessionCloser ends the realtime streams of sessions that just ended.
*/
type SessionCloser interface {
	/*
		CloseSessions disconnects every stream of the sessions.
	*/
	CloseSessions(ctx context.Context, sessionIDs []ids.UUID) error
}

/*
DeleteWidget removes a widget.

Rules:
  - The caller needs widgets:manage.

Events: [appevents.WidgetDeleted].

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a wrapped
database error.
*/
type DeleteWidget struct {
	DB      *database.DB
	Events  *events.Dispatcher
	Clock   clock.Clock
	Streams SessionCloser
	Bust    *Directory
}

/*
Handle removes the widget.
*/
func (a *DeleteWidget) Handle(ctx context.Context, in DeleteWidgetInput) error {
	if !in.Membership.Can("widgets:manage") {
		return exceptions.ErrForbidden
	}

	var sessions []ids.UUID

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		sessions, err = a.remove(ctx, tx, in)

		return err
	})
	if err != nil {
		return err
	}

	a.Bust.Forget(ctx, in.WidgetID)

	return a.Streams.CloseSessions(ctx, sessions)
}

/*
remove writes the deletion and returns the sessions it ended.
*/
func (a *DeleteWidget) remove(ctx context.Context, tx *database.Tx, in DeleteWidgetInput) ([]ids.UUID, error) {
	q, ws, now := models.New(tx), in.Membership.Workspace.ID, a.Clock.Now()

	widget, err := q.FindWidgetSettings(ctx, models.FindWidgetSettingsParams{WorkspaceID: ws, ID: in.WidgetID})
	if errors.Is(err, database.ErrNotFound) {
		return nil, exceptions.ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find widget: %w", err)
	}

	_, err = q.DeleteWidgetSettings(ctx, models.DeleteWidgetSettingsParams{Now: now, WorkspaceID: ws, ID: widget.ID})
	if err != nil {
		return nil, fmt.Errorf("delete widget: %w", err)
	}

	err = q.SetChannelConnectionStatus(ctx, models.SetChannelConnectionStatusParams{
		Status: "disconnected", Now: now, WorkspaceID: ws, ID: widget.ChannelConnectionID,
	})
	if err != nil {
		return nil, fmt.Errorf("disconnect widget channel: %w", err)
	}

	reason := string(enums.WidgetRevokeWidgetDeleted)

	sessions, err := q.RevokeWidgetSessionsForWidget(ctx, models.RevokeWidgetSessionsForWidgetParams{
		Now: now, Reason: reason, WorkspaceID: ws, WidgetSettingsID: widget.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("revoke widget sessions: %w", err)
	}

	return sessions, a.Events.Dispatch(ctx, tx, appevents.WidgetDeleted{
		WorkspaceID: ws, WidgetID: widget.ID, ActorID: in.Membership.Member.ID, Name: widget.Name,
	})
}
