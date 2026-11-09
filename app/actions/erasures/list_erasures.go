/*
|--------------------------------------------------------------------------
| List Erasures
|--------------------------------------------------------------------------
|
| The workspace's privacy requests and where each stands, for the people
| who may answer them.
|
*/

package erasures

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
MaxListed is the most requests one page returns, and TotalCap the most a
count goes up to, so counting never scans a long history.
*/
const (
	MaxListed = 100
	TotalCap  = 10_000
)

/*
ErasureInput is who reads the requests, and which one when reading one.
*/
type ErasureInput struct {
	Membership authz.Membership
	ErasureID  ids.UUID
}

/*
ListErasuresInput is who reads the requests and which page: Status
narrows to one status, Offset skips that many rows, and Count asks how
many match in all, up to TotalCap.
*/
type ListErasuresInput struct {
	Membership authz.Membership
	Status     *string
	Limit      int
	Offset     int
	Count      bool
}

/*
ErasurePage is one page of requests and, when asked, how many match.
*/
type ErasurePage struct {
	Erasures []models.ContactErasure
	Total    *int64
}

/*
ListErasures returns a page of the workspace's requests, newest first.

Rules:
  - The caller needs contacts:erase.
  - A page holds at most MaxListed requests.

Errors: [exceptions.ErrForbidden], or a wrapped database error.
*/
type ListErasures struct {
	DB *database.DB
}

/*
Handle returns the page.
*/
func (a *ListErasures) Handle(ctx context.Context, in ListErasuresInput) (ErasurePage, error) {
	if !in.Membership.Can("contacts:erase") {
		return ErasurePage{}, exceptions.ErrForbidden
	}

	limit := min(max(in.Limit, 1), MaxListed)

	var out ErasurePage

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		var err error

		out.Erasures, err = q.ListContactErasures(ctx, models.ListContactErasuresParams{
			WorkspaceID: in.Membership.Workspace.ID, Status: in.Status,
			RowLimit: int64(limit), RowOffset: int64(in.Offset),
		})
		if err != nil {
			return fmt.Errorf("list erasures: %w", err)
		}

		if !in.Count {
			return nil
		}

		n, err := q.CountContactErasures(ctx, models.CountContactErasuresParams{
			WorkspaceID: in.Membership.Workspace.ID, Status: in.Status, RowLimit: TotalCap,
		})
		if err != nil {
			return fmt.Errorf("count erasures: %w", err)
		}

		out.Total = &n

		return nil
	})

	return out, err
}

/*
ShowErasure returns one request.

Rules:
  - The caller needs contacts:erase.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type ShowErasure struct {
	DB *database.DB
}

/*
Handle returns the request.
*/
func (a *ShowErasure) Handle(ctx context.Context, in ErasureInput) (models.ContactErasure, error) {
	if !in.Membership.Can("contacts:erase") {
		return models.ContactErasure{}, exceptions.ErrForbidden
	}

	var out models.ContactErasure

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		out, err = find(ctx, models.New(tx), in.Membership.Workspace.ID, in.ErasureID)

		return err
	})

	return out, err
}
