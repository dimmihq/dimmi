/*
|--------------------------------------------------------------------------
| List Invitations
|--------------------------------------------------------------------------
|
| Admins see the invitations of their workspace, the newest first, with
| who sent each. An invitation nobody answered before its link ran out
| reads as expired, whether or not anything has marked it so yet.
|
*/

package invitations

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ListInvitationsInput is who lists, through their membership, which
statuses to list, and the page.
*/
type ListInvitationsInput struct {
	Membership authz.Membership
	Statuses   []string
	Limit      int
	Offset     int
}

/*
Page is a page of invitations and whether more follow.
*/
type Page struct {
	Invitations []models.ListInvitationsRow
	More        bool
}

/*
ListInvitations lists the invitations of the workspace in play, newest
first. A pending invitation past its expiry is listed as expired.

Errors: [exceptions.ErrForbidden] without members:write, or a wrapped
database error.
*/
type ListInvitations struct {
	DB    *database.DB
	Clock clock.Clock
}

/*
Handle returns a page of invitations.
*/
func (a *ListInvitations) Handle(ctx context.Context, in ListInvitationsInput) (Page, error) {
	if !in.Membership.Can("members:write") {
		return Page{}, exceptions.ErrForbidden
	}

	var rows []models.ListInvitationsRow

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		rows, err = models.New(tx).ListInvitations(ctx, models.ListInvitationsParams{
			WorkspaceID: in.Membership.Workspace.ID, Statuses: in.Statuses, Now: a.Clock.Now(),
			RowLimit: int64(in.Limit + 1), RowOffset: int64(in.Offset),
		})

		return err
	})
	if err != nil {
		return Page{}, fmt.Errorf("list invitations: %w", err)
	}

	if len(rows) > in.Limit {
		return Page{Invitations: rows[:in.Limit], More: true}, nil
	}

	return Page{Invitations: rows}, nil
}
