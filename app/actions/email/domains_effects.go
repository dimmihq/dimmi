/*
|--------------------------------------------------------------------------
| Sending Domain Effects
|--------------------------------------------------------------------------
|
| What follows a check. Addresses on a verified domain are tied to it, a
| failed domain lets them go, and every check tells open pages to read the
| domain again. A domain taken over from another workspace fails there at
| once, and that workspace's admins hear why. When the install changes
| who sends its mail, every domain is set up again for the new provider
| and verifies anew.
|
*/

package email

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/integrations/email/dns"
	"github.com/dimmihq/dimmi/app/models"
)

/*
apply ties or frees the domain's addresses and tells everyone who
watches what changed.
*/
func (a *CheckEmailDomain) apply(ctx context.Context, tx *database.Tx, before, after models.EmailDomain) error {
	q := models.New(tx)
	now := a.Clock.Now()

	switch {
	case after.Status == DomainVerified && before.Status != DomainVerified:
		err := q.LinkEmailAddressesToDomain(ctx, models.LinkEmailAddressesToDomainParams{
			WorkspaceID: after.WorkspaceID, EmailDomainID: &after.ID, Domain: after.Domain, Now: now,
		})
		if err != nil {
			return fmt.Errorf("link email addresses: %w", err)
		}
	case after.Status == DomainFailed:
		err := q.UnlinkEmailAddressesFromDomain(ctx, models.UnlinkEmailAddressesFromDomainParams{
			WorkspaceID: after.WorkspaceID, EmailDomainID: &after.ID, Now: now,
		})
		if err != nil {
			return fmt.Errorf("unlink email addresses: %w", err)
		}
	}

	return a.announce(ctx, tx, change{from: before.Status, row: after, reason: deref(after.FailureReason)})
}

/*
change is a domain after a change, the state it left and why it left it.
*/
type change struct {
	from   string
	row    models.EmailDomain
	reason string
}

/*
announce dispatches the domain's state change, when there was one, and
the refresh of its records.
*/
func (d *Domains) announce(ctx context.Context, tx *database.Tx, c change) error {
	base := d.base(c.row.WorkspaceID)

	if c.from != c.row.Status {
		err := d.Events.Dispatch(ctx, tx, appevents.EmailDomainStatusChanged{
			Base: base, DomainID: c.row.ID, Domain: c.row.Domain, From: c.from, To: c.row.Status, Reason: c.reason,
		})
		if err != nil {
			return fmt.Errorf("dispatch email domain status: %w", err)
		}
	}

	base.ID = ids.New()

	err := d.Events.Dispatch(ctx, tx, appevents.EmailDomainRecordsUpdated{Base: base, DomainID: c.row.ID})
	if err != nil {
		return fmt.Errorf("dispatch email domain records: %w", err)
	}

	return nil
}

/*
base is the header of an event the system raises in the workspace.
*/
func (d *Domains) base(workspaceID ids.UUID) appevents.Base {
	return appevents.Base{
		ID: ids.New(), WorkspaceID: workspaceID, OccurredAt: d.Clock.Now(),
		Actor: appevents.Actor{Type: "system", ID: ids.Nil()},
	}
}

/*
takeOver fails every other workspace's hold on the domain, so this one
may verify it, and tells those workspaces.
*/
func (a *CheckEmailDomain) takeOver(
	ctx context.Context, tx *database.Tx, row models.EmailDomain, now time.Time,
) error {
	lost, err := models.New(tx).FailEmailDomainClaims(ctx, models.FailEmailDomainClaimsParams{
		Name: row.Domain, KeepWorkspace: row.WorkspaceID, Reason: FailureOwnershipTransfered, At: now,
	})
	if err != nil {
		return fmt.Errorf("fail email domain claims: %w", err)
	}

	for _, claim := range lost {
		err := a.Events.Dispatch(ctx, tx, appevents.EmailDomainStatusChanged{
			Base: a.base(claim.WorkspaceID), DomainID: claim.ID, Domain: row.Domain, From: claim.Status,
			To: DomainFailed, Reason: FailureOwnershipTransfered,
		})
		if err != nil {
			return fmt.Errorf("dispatch email domain status: %w", err)
		}
	}

	return nil
}

/*
finishRotation makes a relay's next key the signing key once its record
is seen everywhere.
*/
func (a *CheckEmailDomain) finishRotation(
	ctx context.Context, q *models.Queries, row models.EmailDomain, found checkOutcome,
) error {
	if row.DkimNextSelector == nil || !nextKeySeen(found.result.Records) {
		return nil
	}

	kept := make([]dns.Record, 0, len(found.result.Records))
	for _, record := range found.result.Records {
		if record.Purpose == PurposeNextDKIM {
			record.Purpose, record.Required = dns.PurposeDKIM, true
		} else if record.Purpose == dns.PurposeDKIM {
			continue
		}

		kept = append(kept, record)
	}

	_, err := q.FinishEmailDomainRotation(ctx, models.FinishEmailDomainRotationParams{
		WorkspaceID: row.WorkspaceID, ID: row.ID, Records: encode(kept), Now: a.Clock.Now(),
	})
	if err != nil {
		return fmt.Errorf("finish signing key rotation: %w", err)
	}

	return nil
}

/*
nextKeySeen reports whether the record of the key taking over verified.
*/
func nextKeySeen(records []dns.Record) bool {
	for _, record := range records {
		if record.Purpose == PurposeNextDKIM {
			return record.Status == dns.StatusVerified
		}
	}

	return false
}

/*
switchDriver sets the domain up for the install's current driver and
starts verifying it again. A domain that was verified sends from the
default sender until it is verified for the new driver.
*/
func (a *CheckEmailDomain) switchDriver(ctx context.Context, tx *database.Tx, row models.EmailDomain) error {
	subdomain := ""
	if row.MailFromDomain != nil {
		subdomain = (*row.MailFromDomain)[:max(0, len(*row.MailFromDomain)-len(row.Domain)-1)]
	}

	set, err := a.setUp(ctx, row, subdomain)
	if err != nil {
		return err
	}

	now := a.Clock.Now()
	next := transition{status: DomainVerifying}
	params := a.checkParams(set, next, now)
	params.Records, params.Warnings, params.VerifyingSince = set.Records, encode([]dns.Warning{}), &now
	params.Driver, params.NextCheckAt = a.Driver, &now
	params.VerifiedAt, params.DegradedAt = nil, nil

	saved, err := models.New(tx).SaveEmailDomainCheck(ctx, params)
	if err != nil {
		return fmt.Errorf("save email domain driver: %w", err)
	}

	return a.announce(ctx, tx, change{from: row.Status, row: saved})
}
