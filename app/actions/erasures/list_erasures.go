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
MaxListed is the most requests a list returns.
*/
const MaxListed = 100

/*
ErasureInput is who reads the requests, and which one when reading one.
*/
type ErasureInput struct {
	Membership authz.Membership
	ErasureID  ids.UUID
}

/*
ListErasures returns the workspace's newest requests.

Rules:
  - The caller needs contacts:erase.

Errors: [exceptions.ErrForbidden], or a wrapped database error.
*/
type ListErasures struct {
	DB *database.DB
}

/*
Handle returns the requests, newest first.
*/
func (a *ListErasures) Handle(ctx context.Context, in ErasureInput) ([]models.ContactErasure, error) {
	if !in.Membership.Can("contacts:erase") {
		return nil, exceptions.ErrForbidden
	}

	var out []models.ContactErasure

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		out, err = models.New(tx).ListContactErasures(ctx, models.ListContactErasuresParams{
			WorkspaceID: in.Membership.Workspace.ID, RowLimit: MaxListed,
		})
		if err != nil {
			return fmt.Errorf("list erasures: %w", err)
		}

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
