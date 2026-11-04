/*
|--------------------------------------------------------------------------
| Managing Sending Domains
|--------------------------------------------------------------------------
|
| Everyone who may read email settings sees the domains and what each is
| missing. Asking for a check, changing the return path and removing a
| domain take email:manage; a new signing key takes email:rotate_dkim,
| which only the owner holds by default. A check is queued, never run in
| the request, and asked for at most once every ten seconds.
|
*/

package email

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/ratelimit"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/integrations/email/dns"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
checkSpacing is how long after one check another may be asked for.
*/
const checkSpacing = 10 * time.Second

/*
ListEmailDomains lists the workspace's live domains by name.

Rules:
  - The caller needs email:read.

Errors: [exceptions.ErrForbidden], or a wrapped database error.
*/
type ListEmailDomains struct {
	DB *database.DB
}

/*
Handle returns the domains.
*/
func (a *ListEmailDomains) Handle(ctx context.Context, m authz.Membership) ([]models.EmailDomain, error) {
	if !m.Can("email:read") {
		return nil, exceptions.ErrForbidden
	}

	var rows []models.EmailDomain

	err := a.DB.Transaction(inWorkspace(ctx, m.Workspace.ID), func(ctx context.Context, tx *database.Tx) error {
		var err error

		rows, err = models.New(tx).ListEmailDomains(ctx, m.Workspace.ID)

		return err
	})
	if err != nil {
		return nil, fmt.Errorf("list email domains: %w", err)
	}

	return rows, nil
}

/*
ShowEmailDomain returns one live domain of the workspace.

Rules:
  - The caller needs email:read.
  - Domains of other workspaces are not found.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type ShowEmailDomain struct {
	DB *database.DB
}

/*
Handle returns the domain.
*/
func (a *ShowEmailDomain) Handle(ctx context.Context, m authz.Membership, id ids.UUID) (models.EmailDomain, error) {
	if !m.Can("email:read") {
		return models.EmailDomain{}, exceptions.ErrForbidden
	}

	return domainIn(ctx, a.DB, m.Workspace.ID, id)
}

/*
domainIn reads a live domain of the workspace in a transaction of its own.
*/
func domainIn(ctx context.Context, db *database.DB, ws, id ids.UUID) (models.EmailDomain, error) {
	var row models.EmailDomain

	err := db.Transaction(inWorkspace(ctx, ws), func(ctx context.Context, tx *database.Tx) error {
		var err error

		row, err = findDomain(ctx, models.New(tx), ws, id)

		return err
	})

	return row, err
}

/*
RequestEmailDomainCheck queues a check of the domain now. A pending or
failed domain starts verifying with a fresh three day window.

Rules:
  - The caller needs email:manage.
  - One request per domain every ten seconds.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrDomainCheckThrottled] with the wait, or a wrapped
database error.
*/
type RequestEmailDomainCheck struct {
	DB      *database.DB
	Queue   *queue.Queue
	Limiter *ratelimit.Limiter
	Clock   clock.Clock
}

/*
Handle queues the check and returns the domain as it now is.
*/
func (a *RequestEmailDomainCheck) Handle(
	ctx context.Context, m authz.Membership, id ids.UUID,
) (models.EmailDomain, error) {
	if !m.Can("email:manage") {
		return models.EmailDomain{}, exceptions.ErrForbidden
	}

	if _, err := domainIn(ctx, a.DB, m.Workspace.ID, id); err != nil {
		return models.EmailDomain{}, err
	}

	result, err := a.Limiter.Hit(ctx, "email_domain_check:"+id.String(), 1, checkSpacing)
	if err != nil {
		return models.EmailDomain{}, fmt.Errorf("limit email domain check: %w", err)
	}

	if !result.Allowed {
		wait := max(1, int((result.RetryAfter+time.Second-1)/time.Second))

		return models.EmailDomain{}, exceptions.ErrDomainCheckThrottled.With("retry_after", wait)
	}

	var row models.EmailDomain

	err = a.DB.Transaction(inWorkspace(ctx, m.Workspace.ID), func(ctx context.Context, tx *database.Tx) error {
		row, err = models.New(tx).RequestEmailDomainCheck(ctx, models.RequestEmailDomainCheckParams{
			WorkspaceID: m.Workspace.ID, ID: id, Now: a.Clock.Now(),
		})
		if err != nil {
			return err
		}

		return a.Queue.PushTx(ctx, tx, jobs.CheckEmailDomainArgs{WorkspaceID: m.Workspace.ID, DomainID: id})
	})
	if err != nil {
		return models.EmailDomain{}, fmt.Errorf("request email domain check: %w", err)
	}

	return row, nil
}

/*
UpdateEmailDomainInput is who asks, the domain, and what changes; a nil
field stays as it is. MailFrom is the return path subdomain, DNSHost the
DNS host the customer says they use, empty to go back to our guess.
*/
type UpdateEmailDomainInput struct {
	Membership authz.Membership
	ID         ids.UUID
	MailFrom   *string
	DNSHost    *string
}

/*
UpdateEmailDomain changes the return path or the DNS host of a domain. A
new return path is set at SES and its records are checked anew; the
domain's other records keep their state.

Rules:
  - The caller needs email:manage.
  - A return path is a single label, such as bounce.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrInvalidDomain], [exceptions.ErrEmailProviderUnavailable],
or a wrapped database error.
*/
type UpdateEmailDomain struct {
	*Domains
	Returns ReturnPathSetter
}

/*
ReturnPathSetter points SES's return path for a domain at a new name.
*/
type ReturnPathSetter interface {
	/*
		SetMailFrom points the domain's return path at the name.
	*/
	SetMailFrom(ctx context.Context, domain, mailFrom string) error
}

/*
Handle applies the changes and returns the domain as it now is.
*/
func (a *UpdateEmailDomain) Handle(ctx context.Context, in UpdateEmailDomainInput) (models.EmailDomain, error) {
	if !in.Membership.Can("email:manage") {
		return models.EmailDomain{}, exceptions.ErrForbidden
	}

	ws := in.Membership.Workspace.ID

	row, err := domainIn(ctx, a.DB, ws, in.ID)
	if err != nil {
		return models.EmailDomain{}, err
	}

	params := models.UpdateEmailDomainSettingsParams{
		WorkspaceID: ws, ID: in.ID, MailFromDomain: row.MailFromDomain, DnsHostOverride: row.DnsHostOverride,
		Records: row.Records, Now: a.Clock.Now(),
	}

	if in.DNSHost != nil {
		params.DnsHostOverride = optional(*in.DNSHost)
	}

	if in.MailFrom != nil && row.Driver == dns.DriverSES {
		if err := a.moveReturnPath(ctx, row, *in.MailFrom, &params); err != nil {
			return models.EmailDomain{}, err
		}
	}

	err = a.DB.Transaction(inWorkspace(ctx, ws), func(ctx context.Context, tx *database.Tx) error {
		row, err = models.New(tx).UpdateEmailDomainSettings(ctx, params)
		if err != nil {
			return err
		}

		return a.announce(ctx, tx, change{from: row.Status, row: row})
	})
	if err != nil {
		return models.EmailDomain{}, fmt.Errorf("update email domain: %w", err)
	}

	return row, nil
}

/*
moveReturnPath points the return path at the subdomain and replaces its
records with fresh ones to check.
*/
func (a *UpdateEmailDomain) moveReturnPath(
	ctx context.Context, row models.EmailDomain, subdomain string, params *models.UpdateEmailDomainSettingsParams,
) error {
	label := strings.ToLower(strings.TrimSpace(subdomain))
	if label == "" || strings.ContainsAny(label, ". @/") {
		return exceptions.ErrInvalidDomain.OnField("mail_from_subdomain")
	}

	mailFrom := label + "." + row.Domain
	if a.Returns != nil {
		if err := a.Returns.SetMailFrom(ctx, row.Domain, mailFrom); err != nil {
			return providerError(err)
		}
	}

	row.MailFromDomain = &mailFrom
	fresh := dns.Expected(a.spec(row, ""))
	kept := make([]dns.Record, 0, len(fresh))

	for _, old := range Records(row) {
		if old.Purpose != dns.PurposeReturnPath && old.Purpose != dns.PurposeReturnPathSPF {
			kept = append(kept, old)
		}
	}

	for _, record := range fresh {
		if record.Purpose == dns.PurposeReturnPath || record.Purpose == dns.PurposeReturnPathSPF {
			kept = append(kept, record)
		}
	}

	params.MailFromDomain, params.Records = &mailFrom, encode(kept)

	return nil
}
