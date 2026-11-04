/*
|--------------------------------------------------------------------------
| Ensure Default Widget
|--------------------------------------------------------------------------
|
| A new workspace has a messenger ready to paste from its first minute, on
| its default inbox, so that the install snippet needs no setup first. Since
| it counts live widgets, deleting the last one stays deleted until someone
| asks for a repair.
|
*/

package widget

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
EnsureDefaultWidget is the workspace initializer that gives a workspace
without a widget its first one. It runs after the default inbox's step.
*/
type EnsureDefaultWidget struct {
	Clock  clock.Clock
	Events *events.Dispatcher
}

/*
Name returns "default_widget".
*/
func (a *EnsureDefaultWidget) Name() string {
	return "default_widget"
}

/*
Initialize creates the widget in the transaction unless the workspace has
one.
*/
func (a *EnsureDefaultWidget) Initialize(ctx context.Context, tx *database.Tx, w models.Workspace) error {
	q := models.New(tx)

	count, err := q.CountWidgetSettings(ctx, w.ID)
	if err != nil {
		return fmt.Errorf("count widgets: %w", err)
	}

	if count > 0 {
		return nil
	}

	inbox, err := findInbox(ctx, q, w.ID, nil)
	if err != nil {
		return err
	}

	widget, err := insertWidget(ctx, q, inbox, DefaultName, a.Clock)
	if err != nil {
		return err
	}

	return a.Events.Dispatch(ctx, tx, appevents.WidgetCreated{
		WorkspaceID: w.ID, WidgetID: widget.ID, ActorID: ids.Nil(),
	})
}
