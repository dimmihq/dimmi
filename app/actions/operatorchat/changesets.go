/*
|--------------------------------------------------------------------------
| Operator Changesets
|--------------------------------------------------------------------------
|
| The changes Operator proposed, for the teammate to apply, decline or
| undo. A teammate sees their own and those Operator proposed on its own
| about conversations they can see; applying needs the right to change
| conversations, and every operation inside still checks its own
| permission as it runs.
|
*/

package operatorchat

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/ai/operator"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
changesetsListed is the most changesets one list shows.
*/
const changesetsListed = 100

/*
ChangesetFilter narrows a list of changesets to one conversation or one
status.
*/
type ChangesetFilter struct {
	Conversation *ids.UUID
	Status       *string
}

/*
Changesets lists and decides Operator's changesets.

Rules:
  - Listing and declining need conversations:read; applying and
    reverting need conversations:write.
  - A changeset is decided once; a failed one may be tried again or
    declined.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[operator.ErrSettled], the error of the operation that failed, or a
wrapped database error.
*/
type Changesets struct {
	DB       *database.DB
	Operator *operator.Engine
}

/*
List returns the changesets the teammate may review, newest first.
*/
func (a *Changesets) List(ctx context.Context, m authz.Membership, f ChangesetFilter,
) ([]models.OperatorChangeset, error) {
	if err := conversations.CanRead(m); err != nil {
		return nil, err
	}
	var out []models.OperatorChangeset
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = models.New(tx).ListOperatorChangesets(ctx, changesetParams(m, changesetFilter{
			conversation: f.Conversation, status: f.Status,
		}))
		if err != nil {
			return fmt.Errorf("list operator changesets: %w", err)
		}
		return nil
	})
	return out, err
}

/*
Apply applies a changeset as the teammate.
*/
func (a *Changesets) Apply(ctx context.Context, m authz.Membership, id ids.UUID) (models.OperatorChangeset, error) {
	if err := conversations.CanWrite(m); err != nil {
		return models.OperatorChangeset{}, err
	}
	return a.Operator.Apply(ctx, m, id)
}

/*
Decline turns a changeset down.
*/
func (a *Changesets) Decline(ctx context.Context, m authz.Membership, id ids.UUID) (models.OperatorChangeset, error) {
	if err := conversations.CanRead(m); err != nil {
		return models.OperatorChangeset{}, err
	}
	return a.Operator.Decline(ctx, m, id)
}

/*
Revert undoes an applied changeset.
*/
func (a *Changesets) Revert(ctx context.Context, m authz.Membership, id ids.UUID) (models.OperatorChangeset, error) {
	if err := conversations.CanWrite(m); err != nil {
		return models.OperatorChangeset{}, err
	}
	return a.Operator.Revert(ctx, m, id)
}

/*
changesetFilter narrows the changeset query.
*/
type changesetFilter struct {
	conversation, thread *ids.UUID
	status               *string
}

/*
changesetParams scopes the changeset query to what the teammate sees.
*/
func changesetParams(m authz.Membership, f changesetFilter) models.ListOperatorChangesetsParams {
	scope := m.ConversationScope().Params()
	return models.ListOperatorChangesetsParams{
		WorkspaceID: m.Workspace.ID, MemberID: &m.Member.ID, ConversationID: f.conversation, ThreadID: f.thread,
		Status: f.status, AccessMode: scope.Mode, AccessViewer: scope.Viewer, AccessTeams: scope.Teams,
		AccessUnassigned: scope.Unassigned, AccessHiddenInboxes: scope.HiddenInboxes, RowLimit: changesetsListed,
	}
}
