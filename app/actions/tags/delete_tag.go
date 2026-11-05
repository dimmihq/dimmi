/*
|--------------------------------------------------------------------------
| Delete Tags
|--------------------------------------------------------------------------
|
| Deleting a tag takes it off every conversation, contact and company in
| one step and says so once, with how many records carried it, so a
| listener never sees a stream of removals for one decision.
|
*/

package tags

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
DeleteTagInput is who removes which tag.
*/
type DeleteTagInput struct {
	Membership authz.Membership
	TagID      ids.UUID
}

/*
DeleteTag removes a tag from the vocabulary and every record carrying
it.

Rules:
  - The caller needs tags:manage.

Events: one [appevents.TagDeleted], with how many records carried it.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type DeleteTag struct {
	DB      *database.DB
	Events  *events.Dispatcher
	Clock   clock.Clock
	Payload Payload
}

/*
Handle removes the tag.
*/
func (a *DeleteTag) Handle(ctx context.Context, in DeleteTagInput) error {
	if !in.Membership.Can("tags:manage") {
		return exceptions.ErrForbidden
	}

	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, ws, now := models.New(tx), in.Membership.Workspace.ID, a.Clock.Now()

		tag, err := find(ctx, q, ws, in.TagID)
		if err != nil {
			return err
		}

		listed := []Listed{{Tag: tag}}
		if err := addCounts(ctx, q, ws, listed); err != nil {
			return err
		}

		if _, err := q.DeleteTag(ctx, models.DeleteTagParams{WorkspaceID: ws, ID: tag.ID}); err != nil {
			return fmt.Errorf("delete tag: %w", err)
		}

		return a.Events.Dispatch(ctx, tx, appevents.TagDeleted{
			Base: base(ws, in.Membership, now), TagID: tag.ID, Conversations: listed[0].Conversations,
			Contacts: listed[0].Contacts, Companies: listed[0].Companies, Data: a.Payload(tag),
		})
	})
}
