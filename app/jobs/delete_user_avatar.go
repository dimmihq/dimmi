/*
|--------------------------------------------------------------------------
| Delete User Avatar
|--------------------------------------------------------------------------
|
| A picture someone replaced or took down, a workspace logo among them,
| or one that belonged to a deleted account, must really go from storage. The job is queued with the change
| itself, so the file goes exactly when the change commits, and storage
| being down only delays it.
|
*/

package jobs

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/support/pictures"
)

/*
DeleteUserAvatarArgs names the picture to remove, a person's or a
workspace's logo.
*/
type DeleteUserAvatarArgs = pictures.DeleteArgs

/*
DeleteUserAvatar removes a picture from storage.
*/
type DeleteUserAvatar struct {
	Disk filesystem.Filesystem
}

/*
Handle removes the file. One already gone is fine.
*/
func (w *DeleteUserAvatar) Handle(ctx context.Context, job *queue.Job[DeleteUserAvatarArgs]) error {
	if err := w.Disk.Delete(ctx, job.Args.Path); err != nil {
		return fmt.Errorf("delete avatar: %w", err)
	}

	return nil
}
