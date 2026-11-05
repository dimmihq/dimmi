/*
|--------------------------------------------------------------------------
| Contact Email Identity
|--------------------------------------------------------------------------
|
| Mail finds its sender through the email identity, and replies go to the
| contact's address, so the two must name the same person. When a
| teammate changes or clears a contact's address, its email identity
| moves with it. An address another contact already receives mail at is
| refused rather than shared, so one address never reaches two people.
|
*/

package contacts

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
emailFree refuses an address another contact of the workspace holds as
its email identity. A contact not yet stored has the zero id.
*/
func emailFree(ctx context.Context, q *models.Queries, self models.Contact, email *string) error {
	if email == nil {
		return nil
	}

	holder, err := q.FindContactByIdentity(ctx, models.FindContactByIdentityParams{
		WorkspaceID: self.WorkspaceID, Type: "email", Value: *email,
	})
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("find email identity: %w", err)
	}

	if holder.ID != self.ID {
		return exceptions.ErrContactEmailTaken
	}

	return nil
}

/*
moveEmail hands the contact's email identity from its old address to
its new one, dropping it when the address was cleared.
*/
func moveEmail(ctx context.Context, q *models.Queries, u *update) error {
	before, next := u.before, u.params.Email
	if sameText(before.Email, next) {
		return nil
	}

	if err := emailFree(ctx, q, before, next); err != nil {
		return err
	}

	if before.Email != nil {
		_, err := q.DeleteContactIdentityByValue(ctx, models.DeleteContactIdentityByValueParams{
			WorkspaceID: before.WorkspaceID, ContactID: before.ID, Type: "email", Value: *before.Email,
		})
		if err != nil {
			return fmt.Errorf("drop email identity: %w", err)
		}
	}

	moved := before
	moved.Email = next

	return claimEmail(ctx, q, moved, u.now)
}
