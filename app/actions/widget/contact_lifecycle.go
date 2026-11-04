/*
|--------------------------------------------------------------------------
| Widget Sessions Across Merges and Erasure
|--------------------------------------------------------------------------
|
| When teammates merge two contacts, an anonymous lead's open messengers
| follow it into the contact that survived, so the visitor keeps talking
| in the same threads. A lead merged into a signed-in customer is a
| different matter: an unproven visitor must never gain a verified
| customer's history, so those sessions end instead. Erasing a contact ends
| every session it had and forgets the pages and context they kept. Both
| can run again with nothing left to do.
|
*/

package widget

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ContactSessions keeps visitors' sessions right when their contact is merged
or erased. Streams closes what ended sessions had open.
*/
type ContactSessions struct {
	Clock   clock.Clock
	Streams SessionCloser
}

/*
MergeInput is a merge inside the caller's transaction: the contact that
went away, the one it became, and whether that one is a signed-in customer.
*/
type MergeInput struct {
	WorkspaceID ids.UUID
	From        ids.UUID
	Into        ids.UUID
	IntoUser    bool
}

/*
Repoint moves the merged lead's live sessions to the contact that survived,
or ends them when it is a signed-in customer, and returns the sessions it
ended so their streams close once the merge commits.
*/
func (a *ContactSessions) Repoint(ctx context.Context, tx *database.Tx, in MergeInput) ([]ids.UUID, error) {
	q := models.New(tx)

	if !in.IntoUser {
		err := q.RepointWidgetSessions(ctx, models.RepointWidgetSessionsParams{
			IntoContactID: in.Into, Now: a.Clock.Now(), WorkspaceID: in.WorkspaceID, ContactID: in.From,
		})
		if err != nil {
			return nil, fmt.Errorf("repoint widget sessions: %w", err)
		}

		return nil, nil
	}

	ended, err := q.RevokeWidgetSessionsForContact(ctx, models.RevokeWidgetSessionsForContactParams{
		Now: a.Clock.Now(), Reason: string(enums.WidgetRevokeContactMerged), WorkspaceID: in.WorkspaceID,
		ContactID: in.From,
	})
	if err != nil {
		return nil, fmt.Errorf("end merged widget sessions: %w", err)
	}

	return ended, nil
}

/*
EraseContact ends every session of the contact, closes their streams and
forgets what they kept about the visitor.
*/
func (a *ContactSessions) EraseContact(ctx context.Context, db *database.DB, workspaceID, contactID ids.UUID) error {
	var ended []ids.UUID

	err := db.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, now := models.New(tx), a.Clock.Now()

		var err error

		ended, err = q.RevokeWidgetSessionsForContact(ctx, models.RevokeWidgetSessionsForContactParams{
			Now: now, Reason: string(enums.WidgetRevokeContactErased), WorkspaceID: workspaceID, ContactID: contactID,
		})
		if err != nil {
			return fmt.Errorf("end erased widget sessions: %w", err)
		}

		err = q.ScrubWidgetSessionsForContact(ctx, models.ScrubWidgetSessionsForContactParams{
			Now: now, WorkspaceID: workspaceID, ContactID: contactID,
		})
		if err != nil {
			return fmt.Errorf("scrub widget sessions: %w", err)
		}

		return nil
	})
	if err != nil || len(ended) == 0 {
		return err
	}

	return a.Streams.CloseSessions(ctx, ended)
}
