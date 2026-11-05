/*
|--------------------------------------------------------------------------
| Changing Email Signatures
|--------------------------------------------------------------------------
|
| A signature is changed or removed by whoever may keep it: its teammate
| with email_signatures:manage_own, anyone with manage_any. A new text
| is checked the same way as when the signature was first kept.
|
*/

package email

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
UpdateEmailSignatureInput is who asks, the signature, and what changes;
a nil field stays as it is.
*/
type UpdateEmailSignatureInput struct {
	Membership authz.Membership
	ID         ids.UUID
	Body       *string
	Enabled    *bool
}

/*
UpdateEmailSignature changes a signature's text or whether it is used.

Rules: the same as for keeping one.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrUnknownSignaturePlaceholder], or a wrapped database error.
*/
type UpdateEmailSignature struct {
	DB    *database.DB
	Clock clock.Clock
}

/*
Handle changes the signature and returns it.
*/
func (a *UpdateEmailSignature) Handle(
	ctx context.Context, in UpdateEmailSignatureInput,
) (models.EmailSignature, error) {
	m := in.Membership

	var row models.EmailSignature

	err := a.DB.Transaction(inWorkspace(ctx, m.Workspace.ID), func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		current, err := ownSignature(ctx, q, m, in.ID)
		if err != nil {
			return err
		}

		if in.Body != nil {
			if err := checkSignature(m, current.WorkspaceMemberID, current.EmailAddressID, *in.Body); err != nil {
				return err
			}
		}

		row, err = q.UpdateEmailSignature(ctx, models.UpdateEmailSignatureParams{
			WorkspaceID: m.Workspace.ID, ID: in.ID, Body: in.Body, Enabled: in.Enabled, Now: a.Clock.Now(),
		})

		return err
	})
	if err != nil {
		return models.EmailSignature{}, fmt.Errorf("update email signature: %w", err)
	}

	return row, nil
}

/*
DeleteEmailSignature removes a signature.

Rules: the same as for keeping one.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type DeleteEmailSignature struct {
	DB *database.DB
}

/*
Handle removes the signature.
*/
func (a *DeleteEmailSignature) Handle(ctx context.Context, m authz.Membership, id ids.UUID) error {
	err := a.DB.Transaction(inWorkspace(ctx, m.Workspace.ID), func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := ownSignature(ctx, q, m, id); err != nil {
			return err
		}

		return q.DeleteEmailSignature(ctx, models.DeleteEmailSignatureParams{WorkspaceID: m.Workspace.ID, ID: id})
	})
	if err != nil {
		return fmt.Errorf("delete email signature: %w", err)
	}

	return nil
}
