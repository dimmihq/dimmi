/*
|--------------------------------------------------------------------------
| Organization Domains
|--------------------------------------------------------------------------
|
| A connection may only sign in addresses under domains its organization
| proved it owns, or any company's identity provider could vouch for
| anyone's account. A company proves a domain by publishing a TXT record
| at _dimmi-verify under it; the first organization to prove a domain
| holds it. Public suffixes and free mail domains are never claimable:
| nobody owns gmail.com. A verified domain then routes sign-ins to one of
| the organization's connections.
|
*/

package sso

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/customdomain"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/dns"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/support/str"

	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/resources/data"
)

/*
Domain statuses and how a domain was proven.
*/
const (
	DomainPending  = "pending"
	DomainVerified = "verified"
	DomainFailed   = "failed"
	ViaDNS         = "dns"
	ViaCLI         = "cli"
)

/*
recordPrefix is the label under a domain that holds its proof.
*/
const recordPrefix = "_dimmi-verify."

/*
proofPrefix starts the value of a domain's proof.
*/
const proofPrefix = "dimmi-domain-verification="

/*
verifiedConstraint is the unique index that lets one organization hold
a verified domain.
*/
const verifiedConstraint = "organization_domains_verified_domain_key"

/*
RecordName returns the name of the TXT record that proves the domain.
*/
func RecordName(domain string) string {
	return recordPrefix + domain
}

/*
Domains holds the domain actions of the organization of the workspace in
play.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrDomainNotClaimable], [exceptions.ErrDomainLimitReached],
[exceptions.ErrDomainTaken], [exceptions.ErrDomainNotVerified],
[exceptions.ErrDomainInUse], or a wrapped error.
*/
type Domains struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
	DNS    *dns.Client
}

/*
Check is what checking a domain came to: the domain as it now is and the
TXT values found at its record.
*/
type Check struct {
	Domain models.OrganizationDomain
	Found  []string
}

/*
List returns the organization's domains by name.

Rules:
  - The caller needs sso:manage.
*/
func (a *Domains) List(ctx context.Context, m authz.Membership) ([]models.OrganizationDomain, error) {
	if !m.Can(managePermission) {
		return nil, exceptions.ErrForbidden
	}

	rows, err := models.New(a.DB).ListOrganizationDomains(ctx, m.Workspace.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("list domains: %w", err)
	}

	return rows, nil
}

/*
Add starts proving a domain, returning it with the token to publish.

Rules:
  - The caller needs sso:manage.
  - The domain is normalized; public suffixes and free mail domains are
    refused.
  - An organization has at most twenty domains; one already added is
    returned as it is.

Events: [appevents.OrganizationDomainChanged] added.
*/
func (a *Domains) Add(ctx context.Context, m authz.Membership, domain string) (models.OrganizationDomain, error) {
	if !m.Can(managePermission) {
		return models.OrganizationDomain{}, exceptions.ErrForbidden
	}

	name, err := claimable(domain)
	if err != nil {
		return models.OrganizationDomain{}, err
	}

	var out models.OrganizationDomain

	err = a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		organizationID := m.Workspace.OrganizationID

		existing, err := q.FindOrganizationDomainByName(ctx, models.FindOrganizationDomainByNameParams{
			OrganizationID: organizationID, Domain: name,
		})
		if err == nil {
			out = existing

			return nil
		}

		if !errors.Is(err, database.ErrNotFound) {
			return fmt.Errorf("find domain: %w", err)
		}

		out, err = a.create(ctx, q, organizationID, name)
		if err != nil {
			return err
		}

		return a.announce(ctx, tx, m, domainChange{out, "added"})
	})

	return out, err
}

/*
create stores a new domain with a fresh token, under the limit.
*/
func (a *Domains) create(
	ctx context.Context, q *models.Queries, organizationID ids.UUID, name string,
) (models.OrganizationDomain, error) {
	count, err := q.CountOrganizationDomains(ctx, organizationID)
	if err != nil {
		return models.OrganizationDomain{}, fmt.Errorf("count domains: %w", err)
	}

	if count >= maxDomains {
		return models.OrganizationDomain{}, exceptions.ErrDomainLimitReached.With("limit", maxDomains)
	}

	created, err := q.CreateOrganizationDomain(ctx, models.CreateOrganizationDomainParams{
		OrganizationID: organizationID, Domain: name, VerificationToken: newToken(), Now: a.Clock.Now(),
	})
	if err != nil {
		return created, fmt.Errorf("add domain: %w", err)
	}

	return created, nil
}

/*
Verify looks the domain's proof up now and records what it found.

Rules:
  - The caller needs sso:manage.
  - A proof found makes the domain verified, unless another organization
    already holds it; none found leaves a pending domain pending.

Events: [appevents.OrganizationDomainChanged] verified.
*/
func (a *Domains) Verify(ctx context.Context, m authz.Membership, id ids.UUID) (Check, error) {
	if !m.Can(managePermission) {
		return Check{}, exceptions.ErrForbidden
	}

	var out Check

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		domain, err := lockDomain(ctx, q, m.Workspace.OrganizationID, id)
		if err != nil {
			return err
		}

		out.Found, err = a.lookup(ctx, domain.Domain)
		if err != nil {
			return err
		}

		out.Domain = domain
		if domain.Status == DomainVerified || !slices.Contains(out.Found, domain.VerificationToken) {
			return nil
		}

		if out.Domain, err = markVerified(ctx, q, domain, proof{ViaDNS, a.Clock.Now()}); err != nil {
			return err
		}

		return a.announce(ctx, tx, m, domainChange{out.Domain, "verified"})
	})

	return out, err
}

/*
Route sends sign-ins under a verified domain to one of the
organization's connections, or to none.

Rules:
  - The caller needs sso:manage.
  - Only a verified domain is routed.
  - A domain routed to an enforced connection is not moved away.

Events: [appevents.OrganizationDomainChanged] routed.
*/
func (a *Domains) Route(
	ctx context.Context, m authz.Membership, id ids.UUID, connection *ids.UUID,
) (models.OrganizationDomain, error) {
	if !m.Can(managePermission) {
		return models.OrganizationDomain{}, exceptions.ErrForbidden
	}

	var out models.OrganizationDomain

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		organizationID := m.Workspace.OrganizationID

		domain, err := lockDomain(ctx, q, organizationID, id)
		if err != nil {
			return err
		}

		if err := a.routable(ctx, q, domain, connection); err != nil {
			return err
		}

		out, err = q.RouteOrganizationDomain(ctx, models.RouteOrganizationDomainParams{
			OrganizationID: organizationID, ID: id, SsoConnectionID: connection, Now: a.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("route domain: %w", err)
		}

		return a.announce(ctx, tx, m, domainChange{out, "routed"})
	})

	return out, err
}

/*
routable refuses to route an unverified domain, to route to another
organization's connection, or to move a domain off an enforced one.
*/
func (a *Domains) routable(
	ctx context.Context, q *models.Queries, domain models.OrganizationDomain, connection *ids.UUID,
) error {
	if domain.Status != DomainVerified {
		return exceptions.ErrDomainNotVerified
	}

	if connection != nil {
		if _, err := lockConnection(ctx, q, domain.OrganizationID, *connection); err != nil {
			return err
		}
	}

	return a.notEnforced(ctx, q, domain)
}

/*
Remove deletes a domain.

Rules:
  - The caller needs sso:manage.
  - A domain routed to an enforced connection is refused.

Events: [appevents.OrganizationDomainChanged] removed.
*/
func (a *Domains) Remove(ctx context.Context, m authz.Membership, id ids.UUID) error {
	if !m.Can(managePermission) {
		return exceptions.ErrForbidden
	}

	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		organizationID := m.Workspace.OrganizationID

		domain, err := lockDomain(ctx, q, organizationID, id)
		if err != nil {
			return err
		}

		if err := a.notEnforced(ctx, q, domain); err != nil {
			return err
		}

		if _, err := q.DeleteOrganizationDomain(ctx, models.DeleteOrganizationDomainParams{
			OrganizationID: organizationID, ID: id,
		}); err != nil {
			return fmt.Errorf("remove domain: %w", err)
		}

		return a.announce(ctx, tx, m, domainChange{domain, "removed"})
	})
}

/*
notEnforced refuses a change to a domain routed to an enforced
connection.
*/
func (a *Domains) notEnforced(ctx context.Context, q *models.Queries, domain models.OrganizationDomain) error {
	if domain.SsoConnectionID == nil {
		return nil
	}

	routed, err := lockConnection(ctx, q, domain.OrganizationID, *domain.SsoConnectionID)
	if err != nil && !errors.Is(err, exceptions.ErrNotFound) {
		return err
	}

	if err == nil && routed.Enforced {
		return exceptions.ErrDomainInUse
	}

	return nil
}

/*
lookup returns the TXT values published at the domain's record, none
when the name does not exist.
*/
func (a *Domains) lookup(ctx context.Context, domain string) ([]string, error) {
	found, err := a.DNS.TXT(ctx, RecordName(domain))
	if errors.Is(err, dns.ErrNotFound) {
		return []string{}, nil
	}

	if err != nil {
		return nil, fmt.Errorf("look up domain proof: %w", err)
	}

	return found, nil
}

/*
announce dispatches a change to a domain, from the member's workspace.
*/
func (a *Domains) announce(ctx context.Context, tx *database.Tx, m authz.Membership, change domainChange) error {
	return a.Events.Dispatch(ctx, tx, appevents.OrganizationDomainChanged{
		WorkspaceID: m.Workspace.ID, DomainID: change.domain.ID, Domain: change.domain.Domain, Change: change.kind,
		Changes: map[string]any{"status": change.domain.Status}, ChangedBy: m.Member.ID,
	})
}

/*
domainChange is what happened to a domain: the domain and the kind of
change.
*/
type domainChange struct {
	domain models.OrganizationDomain
	kind   string
}

/*
claimable returns the domain's canonical form, refusing what no single
company can own.
*/
func claimable(domain string) (string, error) {
	name, err := customdomain.Normalize(domain)
	if err != nil {
		return "", exceptions.ErrDomainNotClaimable
	}

	if _, free := data.FreemailDomains()[name]; free {
		return "", exceptions.ErrDomainNotClaimable
	}

	return name, nil
}

/*
proof is how and when a domain was proven.
*/
type proof struct {
	via string
	at  time.Time
}

/*
markVerified records the domain proven, refusing one another
organization already holds.
*/
func markVerified(
	ctx context.Context, q *models.Queries, domain models.OrganizationDomain, by proof,
) (models.OrganizationDomain, error) {
	via, now := by.via, by.at

	verified, err := q.MarkOrganizationDomainChecked(ctx, models.MarkOrganizationDomainCheckedParams{
		OrganizationID: domain.OrganizationID, ID: domain.ID, Status: DomainVerified, VerifiedAt: &now,
		VerifiedVia: &via, Now: now,
	})
	if database.IsUniqueViolation(err, verifiedConstraint) {
		return domain, exceptions.ErrDomainTaken
	}

	if err != nil {
		return domain, fmt.Errorf("verify domain: %w", err)
	}

	return verified, nil
}

/*
lockDomain returns the organization's domain and holds its row, or
exceptions.ErrNotFound.
*/
func lockDomain(
	ctx context.Context, q *models.Queries, organizationID, id ids.UUID,
) (models.OrganizationDomain, error) {
	domain, err := q.LockOrganizationDomain(ctx, models.LockOrganizationDomainParams{
		OrganizationID: organizationID, ID: id,
	})
	if errors.Is(err, database.ErrNotFound) {
		return domain, exceptions.ErrNotFound
	}

	if err != nil {
		return domain, fmt.Errorf("lock domain: %w", err)
	}

	return domain, nil
}

/*
newToken returns a fresh proof value: the prefix and 32 random letters
and digits.
*/
func newToken() string {
	return proofPrefix + str.Random(32)
}
