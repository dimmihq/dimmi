/*
|--------------------------------------------------------------------------
| Recheck Organization Domains
|--------------------------------------------------------------------------
|
| A domain proven by DNS must keep its proof: a domain that lapsed or
| changed hands must not keep routing sign-ins to its old owner. Every day
| each such domain is looked up again. A proof missing for a week fails
| the domain, stops enforcing the connection it routes to, and tells the
| owners. A blip in between changes nothing. Domains an operator vouched
| for are never looked up.
|
*/

package jobs

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/dns"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/actions/sso"
	appauth "github.com/dimmihq/dimmi/app/auth"
	appevents "github.com/dimmihq/dimmi/app/events"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
)

/*
domainGrace is how long a proof may be missing before its domain fails.
*/
const domainGrace = 7 * 24 * time.Hour

/*
RecheckOrganizationDomainsArgs carries nothing: every run checks every
domain proven by DNS.
*/
type RecheckOrganizationDomainsArgs struct{}

/*
Kind names the job.
*/
func (RecheckOrganizationDomainsArgs) Kind() string {
	return "recheck_organization_domains"
}

/*
Options try the job a few times; the next day's run catches up anyway.
*/
func (RecheckOrganizationDomainsArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: expiryAttempts}
}

/*
RecheckOrganizationDomains looks every domain proven by DNS up again.
*/
type RecheckOrganizationDomains struct {
	DB     *database.DB
	Clock  clock.Clock
	DNS    *dns.Client
	Mail   *mail.Manager
	Events *events.Dispatcher
	Links  appauth.Links
}

/*
Handle checks each domain on its own, so one failure does not hold up
the rest.
*/
func (w *RecheckOrganizationDomains) Handle(ctx context.Context, _ *queue.Job[RecheckOrganizationDomainsArgs]) error {
	domains, err := models.New(w.DB).ListDomainsToRecheck(ctx)
	if err != nil {
		return fmt.Errorf("list domains to recheck: %w", err)
	}

	var errs []error

	for _, domain := range domains {
		if err := w.recheck(ctx, domain); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

/*
recheck looks the domain's proof up and records what it found.
*/
func (w *RecheckOrganizationDomains) recheck(ctx context.Context, domain models.OrganizationDomain) error {
	found, err := w.DNS.TXT(ctx, sso.RecordName(domain.Domain))
	if err != nil && !errors.Is(err, dns.ErrNotFound) {
		return fmt.Errorf("look up %s: %w", domain.Domain, err)
	}

	now := w.Clock.Now()
	next := models.MarkOrganizationDomainCheckedParams{
		OrganizationID: domain.OrganizationID, ID: domain.ID, Status: domain.Status, VerifiedAt: domain.VerifiedAt,
		VerifiedVia: domain.VerifiedVia, Now: now,
	}

	if slices.Contains(found, domain.VerificationToken) {
		next.Status = sso.DomainVerified
	} else {
		since := now
		if domain.FailedSince != nil {
			since = *domain.FailedSince
		}

		next.FailedSince = &since
		if now.Sub(since) >= domainGrace {
			next.Status = sso.DomainFailed
		}
	}

	return w.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		if _, err := q.MarkOrganizationDomainChecked(ctx, next); err != nil {
			return fmt.Errorf("record domain check: %w", err)
		}

		if next.Status != sso.DomainFailed || domain.Status == sso.DomainFailed {
			return nil
		}

		return w.fail(ctx, tx, domain)
	})
}

/*
fail stops enforcing the connection the domain routes to and tells the
owners, recorded as the system.
*/
func (w *RecheckOrganizationDomains) fail(
	ctx context.Context, tx *database.Tx, domain models.OrganizationDomain,
) error {
	q := models.New(tx)
	now := w.Clock.Now()

	if domain.SsoConnectionID != nil {
		if err := q.UnenforceSSOConnection(ctx, models.UnenforceSSOConnectionParams{
			OrganizationID: domain.OrganizationID, ID: *domain.SsoConnectionID, Now: now,
		}); err != nil {
			return fmt.Errorf("unenforce connection: %w", err)
		}

		if err := sso.Mirror(ctx, q, domain.OrganizationID, now); err != nil {
			return err
		}
	}

	owners, err := q.ListOrganizationOwners(ctx, domain.OrganizationID)
	if err != nil {
		return fmt.Errorf("list owners to tell: %w", err)
	}

	if len(owners) == 0 {
		return nil
	}

	if err := w.Events.Dispatch(ctx, tx, appevents.OrganizationDomainChanged{
		WorkspaceID: owners[0].WorkspaceID, DomainID: domain.ID, Domain: domain.Domain, Change: "failed",
		Changes: map[string]any{"status": sso.DomainFailed}, ChangedBy: ids.Nil(),
	}); err != nil {
		return err
	}

	for _, owner := range owners {
		if err := w.tell(ctx, tx, owner, domain.Domain); err != nil {
			return err
		}
	}

	return nil
}

/*
tell queues the owner's mail about the domain that lost its proof.
*/
func (w *RecheckOrganizationDomains) tell(
	ctx context.Context, tx *database.Tx, owner models.ListOrganizationOwnersRow, domain string,
) error {
	pending := w.Mail.To(mail.Address{Name: owner.Name, Email: owner.Email})
	if owner.Locale != nil && *owner.Locale != "" {
		pending.Locale(*owner.Locale)
	}

	err := pending.QueueTx(ctx, tx, appmail.DomainVerificationLostMail{
		Name: owner.Name, Domain: domain, SettingsURL: w.Links.SingleSignOn(),
	})
	if err != nil {
		return fmt.Errorf("queue domain lost mail: %w", err)
	}

	return nil
}
