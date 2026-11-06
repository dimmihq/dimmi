/*
|--------------------------------------------------------------------------
| Workspace Logo
|--------------------------------------------------------------------------
|
| The logo stands for the workspace in front of customers, so only people
| who may change settings change it. It is kept the way every picture is,
| as a small square under a new name each time, and the old file goes
| after the change commits, by a job that can try again. Removing a logo
| twice changes nothing.
|
*/

package workspaces

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/pictures"
)

/*
UpdateLogo replaces the workspace's logo.

Rules:
  - The caller needs settings:write.

Events: [appevents.WorkspaceUpdated].

Errors: [exceptions.ErrForbidden], [exceptions.ErrInvalidAvatar],
[exceptions.ErrAvatarTooLarge], or a wrapped error.
*/
type UpdateLogo struct {
	DB     *database.DB
	Clock  clock.Clock
	Events *events.Dispatcher
	Disk   filesystem.Filesystem
	Queue  *queue.Queue
}

/*
Handle stores the logo and returns the workspace as it now is.
*/
func (a *UpdateLogo) Handle(
	ctx context.Context, membership authz.Membership, upload io.Reader,
) (models.Workspace, error) {
	if !membership.Can("settings:write") {
		return models.Workspace{}, exceptions.ErrForbidden
	}

	path, err := pictures.Store(ctx, a.Disk, upload)
	if err != nil {
		return models.Workspace{}, err
	}

	writer := logoWriter{DB: a.DB, Clock: a.Clock, Events: a.Events, Queue: a.Queue}

	workspace, err := writer.replace(ctx, membership, &path)
	if err != nil {
		return models.Workspace{}, errors.Join(err, a.Disk.Delete(ctx, path))
	}

	return workspace, nil
}

/*
DeleteLogo removes the workspace's logo.

Rules:
  - The caller needs settings:write.

Events: [appevents.WorkspaceUpdated] when there was one.

Errors: [exceptions.ErrForbidden], or a wrapped error.
*/
type DeleteLogo struct {
	DB     *database.DB
	Clock  clock.Clock
	Events *events.Dispatcher
	Queue  *queue.Queue
}

/*
Handle removes the logo.
*/
func (a *DeleteLogo) Handle(ctx context.Context, membership authz.Membership) error {
	if !membership.Can("settings:write") {
		return exceptions.ErrForbidden
	}

	writer := logoWriter{DB: a.DB, Clock: a.Clock, Events: a.Events, Queue: a.Queue}
	_, err := writer.replace(ctx, membership, nil)

	return err
}

/*
logoWriter records which logo a workspace has.
*/
type logoWriter struct {
	DB     *database.DB
	Clock  clock.Clock
	Events *events.Dispatcher
	Queue  *queue.Queue
}

/*
replace gives the workspace the logo at the path, or none, queues the
old file's removal and tells the team, in one transaction.
*/
func (w logoWriter) replace(
	ctx context.Context, membership authz.Membership, path *string,
) (models.Workspace, error) {
	var updated models.Workspace

	err := w.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		current, err := q.LockWorkspace(ctx, membership.Workspace.ID)
		if err != nil {
			return fmt.Errorf("lock workspace to change logo: %w", err)
		}

		updated, err = q.ReplaceWorkspaceLogoPath(ctx, models.ReplaceWorkspaceLogoPathParams{
			ID: current.ID, LogoPath: path, Now: w.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("record logo: %w", err)
		}

		if current.LogoPath == nil && path == nil {
			return nil
		}

		if current.LogoPath != nil {
			err = w.Queue.PushTx(ctx, tx, pictures.DeleteArgs{Path: *current.LogoPath})
			if err != nil {
				return fmt.Errorf("queue removal of old logo: %w", err)
			}
		}

		return w.Events.Dispatch(ctx, tx, appevents.WorkspaceUpdated{
			WorkspaceID: current.ID, UpdatedBy: membership.Member.ID, Changes: []string{"logo"},
		})
	})

	return updated, err
}
