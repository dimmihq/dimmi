/*
|--------------------------------------------------------------------------
| Removing a Domain and Rotating its Key
|--------------------------------------------------------------------------
|
| Removing a domain addresses still send from would quietly move their
| mail to the default sender, so it is refused with the list of those
| addresses until the admin says to go ahead. The row stays, soft
| deleted, for the record; the provider's copy is removed, and a provider
| that fails to answer never keeps the domain in the workspace.
|
| A relay's signing key is rotated by publishing a new key beside the old
| one: the old key keeps signing until the new record is seen, so no mail
| ever goes out signed with a key nobody can look up.
|
*/

package email

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/log"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/integrations/email/dns"
	"github.com/dimmihq/dimmi/app/models"
)

/*
DeleteEmailDomainInput is who asks, the domain, and whether addresses
still on it may fall back to the default sender.
*/
type DeleteEmailDomainInput struct {
	Membership authz.Membership
	ID         ids.UUID
	Force      bool
}

/*
DeleteEmailDomain removes a domain.

Rules:
  - The caller needs email:manage.
  - A verified domain addresses still send from is only removed with
    Force; its addresses then send from the default sender.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrDomainInUse] listing the addresses, or a wrapped
database error.
*/
type DeleteEmailDomain struct {
	*Domains
}

/*
Handle removes the domain.
*/
func (a *DeleteEmailDomain) Handle(ctx context.Context, in DeleteEmailDomainInput) error {
	if !in.Membership.Can("email:manage") {
		return exceptions.ErrForbidden
	}

	ws := in.Membership.Workspace.ID

	var row models.EmailDomain

	err := a.DB.Transaction(inWorkspace(ctx, ws), func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		var err error

		row, err = findDomain(ctx, q, ws, in.ID)
		if err != nil {
			return err
		}

		if !in.Force {
			if err := a.refuseInUse(ctx, q, row); err != nil {
				return err
			}
		}

		return a.remove(ctx, tx, row)
	})
	if err != nil {
		return err
	}

	if a.Provider != nil {
		if err := a.Provider.Remove(ctx, row); err != nil {
			log.From(ctx).WarnContext(ctx, "email domain left at the provider",
				log.String("domain", row.Domain), log.Err(err))
		}
	}

	return nil
}

/*
refuseInUse refuses removing a verified domain addresses send from.
*/
func (a *DeleteEmailDomain) refuseInUse(ctx context.Context, q *models.Queries, row models.EmailDomain) error {
	if row.Status != DomainVerified {
		return nil
	}

	addresses, err := q.ListEmailDomainAddresses(ctx, models.ListEmailDomainAddressesParams{
		WorkspaceID: row.WorkspaceID, EmailDomainID: &row.ID,
	})
	if err != nil {
		return fmt.Errorf("list email domain addresses: %w", err)
	}

	if len(addresses) > 0 {
		return exceptions.ErrDomainInUse.With("addresses", addresses)
	}

	return nil
}

/*
remove soft deletes the row, frees its addresses and tells open pages.
*/
func (a *DeleteEmailDomain) remove(ctx context.Context, tx *database.Tx, row models.EmailDomain) error {
	q := models.New(tx)
	now := a.Clock.Now()

	err := q.UnlinkEmailAddressesFromDomain(ctx, models.UnlinkEmailAddressesFromDomainParams{
		WorkspaceID: row.WorkspaceID, EmailDomainID: &row.ID, Now: now,
	})
	if err != nil {
		return fmt.Errorf("unlink email addresses: %w", err)
	}

	if err := q.DeleteEmailDomain(ctx, models.DeleteEmailDomainParams{
		WorkspaceID: row.WorkspaceID, ID: row.ID, Now: now,
	}); err != nil {
		return fmt.Errorf("delete email domain: %w", err)
	}

	return a.announce(ctx, tx, change{from: row.Status, row: row})
}

/*
RotateEmailDomainKey publishes a new signing key for a relay's domain.
The current key signs until the new record is seen; then the new one
takes over and the old record may go after a week.

Rules:
  - The caller needs email:rotate_dkim.
  - Only domains we sign for have a key to rotate; SES rotates its own.
  - A rotation already waiting for its record is answered as it is.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrDKIMRotationUnsupported], or a wrapped error.
*/
type RotateEmailDomainKey struct {
	*Domains
}

/*
Handle starts the rotation and returns the domain with the new record.
*/
func (a *RotateEmailDomainKey) Handle(
	ctx context.Context, m authz.Membership, id ids.UUID,
) (models.EmailDomain, error) {
	if !m.Can("email:rotate_dkim") {
		return models.EmailDomain{}, exceptions.ErrForbidden
	}

	row, err := domainIn(ctx, a.DB, m.Workspace.ID, id)
	if err != nil {
		return models.EmailDomain{}, err
	}

	if row.Driver != dns.DriverSMTP {
		return models.EmailDomain{}, exceptions.ErrDKIMRotationUnsupported
	}

	if row.DkimNextSelector != nil {
		return row, nil
	}

	key, err := a.newRelayKey(ctx, row.WorkspaceID, row.Domain, a.Clock.Now().Format("200601"))
	if err != nil {
		return models.EmailDomain{}, err
	}

	upcoming := row
	upcoming.DkimSelector = &key.selector
	record := dns.Expected(a.spec(upcoming, key.public))[1]
	record.Purpose, record.Required = PurposeNextDKIM, false

	err = a.DB.Transaction(inWorkspace(ctx, row.WorkspaceID), func(ctx context.Context, tx *database.Tx) error {
		row, err = models.New(tx).StartEmailDomainRotation(ctx, models.StartEmailDomainRotationParams{
			WorkspaceID: row.WorkspaceID, ID: row.ID, DkimNextSelector: &key.selector,
			DkimNextPrivateKeyEncrypted: key.sealed, Records: encode(append(Records(row), record)),
			Now: a.Clock.Now(),
		})
		if err != nil {
			return err
		}

		return a.announce(ctx, tx, change{from: row.Status, row: row})
	})
	if err != nil {
		return models.EmailDomain{}, fmt.Errorf("rotate signing key: %w", err)
	}

	return row, nil
}
