/*
|--------------------------------------------------------------------------
| Email Suppressions
|--------------------------------------------------------------------------
|
| The addresses the workspace will not mail, as a teammate sees them on a
| customer: why mail stopped and since when. Lifting a suppression lets
| mail reach the address again. A complaint was the customer's own
| choice, so lifting one takes its own permission.
|
*/

package email

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ListContactSuppressions returns the live suppressions on a contact's
address.

Rules:
  - The caller needs contacts:read.
  - A contact of another workspace has none.

Errors: [exceptions.ErrForbidden], or a wrapped database error.
*/
type ListContactSuppressions struct {
	DB    *database.DB
	Clock clock.Clock
}

/*
Handle returns the contact's suppressions, newest first.
*/
func (a *ListContactSuppressions) Handle(
	ctx context.Context, m authz.Membership, contactID ids.UUID,
) ([]models.EmailSuppression, error) {
	if !m.Can("contacts:read") {
		return nil, exceptions.ErrForbidden
	}

	var rows []models.EmailSuppression

	err := a.DB.Transaction(inWorkspace(ctx, m.Workspace.ID), func(ctx context.Context, tx *database.Tx) error {
		var err error

		rows, err = models.New(tx).ListContactSuppressions(ctx, models.ListContactSuppressionsParams{
			WorkspaceID: m.Workspace.ID, ContactID: contactID, Now: a.Clock.Now(),
		})

		return err
	})
	if err != nil {
		return nil, fmt.Errorf("list contact suppressions: %w", err)
	}

	return rows, nil
}

/*
LiftSuppression lets mail reach a suppressed address again.

Rules:
  - The caller needs email_suppressions:manage, or
    email_suppressions:lift_complaint when the address complained.
  - Suppressions of other workspaces are not found.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type LiftSuppression struct {
	DB *database.DB
}

/*
Handle removes the suppression.
*/
func (a *LiftSuppression) Handle(ctx context.Context, m authz.Membership, id ids.UUID) error {
	if !m.Can("email_suppressions:manage") && !m.Can("email_suppressions:lift_complaint") {
		return exceptions.ErrForbidden
	}

	return a.DB.Transaction(inWorkspace(ctx, m.Workspace.ID), func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		row, err := q.FindEmailSuppression(ctx, models.FindEmailSuppressionParams{WorkspaceID: m.Workspace.ID, ID: id})
		if errors.Is(err, database.ErrNotFound) {
			return exceptions.ErrNotFound
		}

		if err != nil {
			return fmt.Errorf("find email suppression: %w", err)
		}

		need := "email_suppressions:manage"
		if row.Reason == "complaint" {
			need = "email_suppressions:lift_complaint"
		}

		if !m.Can(need) {
			return exceptions.ErrForbidden
		}

		err = q.DeleteEmailSuppression(ctx, models.DeleteEmailSuppressionParams{WorkspaceID: m.Workspace.ID, ID: id})
		if err != nil {
			return fmt.Errorf("delete email suppression: %w", err)
		}

		return nil
	})
}
