/*
|--------------------------------------------------------------------------
| Erasure Helpers
|--------------------------------------------------------------------------
|
| What the erasure actions share: the event header of a member's change,
| the keys the stash is sealed with, and finding a request.
|
*/

package erasures

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
base returns the event header for a change the member made.
*/
func base(m authz.Membership, now time.Time) appevents.Base {
	return appevents.Base{
		ID: ids.New(), WorkspaceID: m.Workspace.ID, OccurredAt: now,
		Actor: appevents.Actor{Type: "member", ID: m.Member.ID},
	}
}

/*
systemBase returns the event header for a step the erase job took.
*/
func systemBase(workspaceID ids.UUID, now time.Time) appevents.Base {
	return appevents.Base{
		ID: ids.New(), WorkspaceID: workspaceID, OccurredAt: now, Actor: appevents.Actor{Type: "system", ID: ids.Nil()},
	}
}

/*
scope is the keys of the workspace the stash is sealed with.
*/
func scope(workspaceID ids.UUID) encryption.Scope {
	return encryption.Scope{Kind: "workspace", ID: workspaceID.String()}
}

/*
stashData binds a stash to the contact it belongs to.
*/
func stashData(contactID ids.UUID) []byte {
	return []byte("contact_erasures.identities_stash:" + contactID.String())
}

/*
find returns the request, or exceptions.ErrNotFound.
*/
func find(ctx context.Context, q *models.Queries, ws, id ids.UUID) (models.ContactErasure, error) {
	row, err := q.FindContactErasure(ctx, models.FindContactErasureParams{WorkspaceID: ws, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return row, exceptions.ErrNotFound
	}

	if err != nil {
		return row, fmt.Errorf("find erasure: %w", err)
	}

	return row, nil
}

/*
open reports whether the request can still be carried out or canceled.
*/
func open(row models.ContactErasure) bool {
	return row.Status == "pending" || row.Status == "pending_hold"
}
