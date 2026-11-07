/*
|--------------------------------------------------------------------------
| Forget Workspace Widgets
|--------------------------------------------------------------------------
|
| A messenger shows its workspace's name and logo, which live on the
| workspace rather than the widget. When either changes, every replica
| forgets the workspace's widgets, so the next visitor gets the new name
| and logo without waiting for the copies to expire.
|
*/

package widget

import (
	"context"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/models"
)

/*
ForgetWorkspaceWidgets drops a workspace's widgets from every replica's
directory.
*/
type ForgetWorkspaceWidgets struct {
	DB   *database.DB
	Bust *Directory
}

/*
Handle forgets the widgets when the changes touch what a messenger shows
of its workspace.
*/
func (a *ForgetWorkspaceWidgets) Handle(ctx context.Context, workspace ids.UUID, changes []string) error {
	if !slices.Contains(changes, "name") && !slices.Contains(changes, "logo") {
		return nil
	}

	return a.ForgetAll(ctx, workspace)
}

/*
ForgetAll forgets every widget of the workspace, whatever changed.
*/
func (a *ForgetWorkspaceWidgets) ForgetAll(ctx context.Context, workspace ids.UUID) error {
	var widgets []models.WidgetSetting

	ctx = tenancy.With(ctx, workspace.String())
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		widgets, err = models.New(tx).ListWidgetSettings(ctx, workspace)

		return err
	})
	if err != nil {
		return fmt.Errorf("list widgets to forget: %w", err)
	}

	for _, widget := range widgets {
		a.Bust.Forget(ctx, widget.ID)
	}

	return nil
}
