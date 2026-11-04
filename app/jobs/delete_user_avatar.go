/*
|--------------------------------------------------------------------------
| Delete User Avatar
|--------------------------------------------------------------------------
|
| A picture someone replaced or took down, or that belonged to a deleted
| account, must really go from storage. The job is queued with the change
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
)

/*
DeleteUserAvatarArgs names the file to remove.
*/
type DeleteUserAvatarArgs struct {
	Path string `json:"path"`
}

/*
Kind names the job.
*/
func (DeleteUserAvatarArgs) Kind() string {
	return "delete_user_avatar"
}

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
