/*
|--------------------------------------------------------------------------
| Check Workspace Owners
|--------------------------------------------------------------------------
|
| Every live workspace needs an active owner, someone who can run it.
| The daily round finds the ones left without one, such as when the last
| owner deleted their account, and promotes someone. A workspace that
| fails does not stop the others.
|
*/

package jobs

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/actions/members"
	"github.com/dimmihq/dimmi/app/models"
)

/*
CheckWorkspaceOwnersArgs starts the daily round that makes sure every
live workspace has an active owner.
*/
type CheckWorkspaceOwnersArgs struct{}

/*
Kind names the job.
*/
func (CheckWorkspaceOwnersArgs) Kind() string {
	return "check_workspace_owners"
}

/*
CheckWorkspaceOwners gives every live workspace without an active owner
one, through [members.EnsureWorkspaceHasOwner]. A workspace that fails
does not stop the others; the round reports every failure once it went
through all of them.
*/
type CheckWorkspaceOwners struct {
	DB     *database.DB
	Ensure *members.EnsureWorkspaceHasOwner
}

/*
Handle checks every live workspace.
*/
func (w *CheckWorkspaceOwners) Handle(ctx context.Context, _ *queue.Job[CheckWorkspaceOwnersArgs]) error {
	live, err := models.New(w.DB).ListWorkspaces(ctx)
	if err != nil {
		return fmt.Errorf("list workspaces: %w", err)
	}

	var failures []error

	for _, workspace := range live {
		if _, err := w.Ensure.Handle(ctx, workspace.ID); err != nil {
			failures = append(failures, fmt.Errorf("workspace %s: %w", workspace.ID, err))
		}
	}

	return errors.Join(failures...)
}
