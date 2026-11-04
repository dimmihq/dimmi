/*
|--------------------------------------------------------------------------
| Retention Guard
|--------------------------------------------------------------------------
|
| Data is destroyed in one place per kind of data: a workspace purge, an
| audit log prune, a contact erasure. Each asks the guard first, so a
| legal hold or a retention policy is one new guard rather than an edit
| to every job that deletes. A held subject is skipped and asked about
| again on the job's next run; nothing is lost while the hold lasts.
|
*/

package contracts

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
RetentionWorkspace is the kind of subject a workspace purge asks about.
*/
const RetentionWorkspace = "workspace"

/*
RetentionSubject is what a job is about to destroy: Kind says what it
is, such as "workspace", and ID which one.
*/
type RetentionSubject struct {
	Kind string
	ID   ids.UUID
}

/*
RetentionGuard decides whether data may be destroyed.
*/
type RetentionGuard interface {
	/*
		MayDestroy reports whether the subject, which belongs to the
		workspace, may be destroyed now. False holds it: the job skips
		it and records that it was held.
	*/
	MayDestroy(ctx context.Context, workspaceID ids.UUID, subject RetentionSubject) (bool, error)
}
