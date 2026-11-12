/*
|--------------------------------------------------------------------------
| Help Center Domains
|--------------------------------------------------------------------------
|
| A help center may answer on a domain the workspace owns. The team adds
| it and gets two records to create: a CNAME pointing the domain at the
| install, and a TXT record holding a token only they were given, which
| proves they own the domain. A check in the background then looks until
| both hold, and only then does the help center answer there, get a
| certificate, and name the domain in its canonical links. One domain per
| help center; a domain answers for one help center across the install.
|
*/

package helpcenters

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/customdomain"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/dns"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/support/str"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The states of a help center's domain.
*/
const (
	DomainPending  = "pending"
	DomainVerified = "verified"
	DomainFailed   = "failed"
)

/*
domainRecordPrefix names the ownership record under the domain, and
domainValuePrefix starts the value it holds.
*/
const (
	domainRecordPrefix = "_dimmi."
	domainValuePrefix  = "dimmi-verification="
)

/*
Failures of managing a help center's domain.
*/
var (
	ErrDomainInvalid = fault.New(fault.Invalid, "domain_invalid",
		"Use a domain you own, such as help.example.com.").OnField("hostname")
	ErrDomainReserved = fault.New(fault.Invalid, "domain_reserved",
		"That domain belongs to dimmi itself; use one you own.").OnField("hostname")
	ErrDomainTaken = fault.New(fault.Conflict, "domain_taken",
		"Another help center already uses this domain.").OnField("hostname")
	ErrDomainExists = fault.New(fault.Conflict, "domain_exists",
		"This help center already has a domain. Remove it first.").OnField("hostname")
	ErrDomainNotFound = fault.New(fault.NotFound, "domain_not_found", "This help center has no custom domain.")
	ErrDomainApex     = fault.New(fault.Invalid, "domain_apex",
		"Use a subdomain such as help.example.com; a bare domain cannot point here with a CNAME.").
		OnField("hostname")
)

/*
DomainVerifier checks a hostname's records; customdomain.Verifier is one.
*/
type DomainVerifier interface {
	/*
		Check reports what holds of the hostname's records.
	*/
	Check(ctx context.Context, host string, expect customdomain.Expect) (customdomain.Result, error)
}

/*
Domains manages the domains help centers answer on. Target is the
hostname their CNAME points at, Reserved the hosts of the install
itself no workspace may claim, and TLS, when set, is told when a domain
is verified or removed. Probe answers the routing probe the checks run,
and Resolver looks up the target's own addresses.
*/
type Domains struct {
	DB       *database.DB
	Clock    clock.Clock
	Verifier DomainVerifier
	Target   string
	Reserved []string
	TLS      customdomain.TLS
	Schedule customdomain.Schedule
	Probe    customdomain.Probe
	Resolver dns.Resolver
}

/*
Domain is a help center's domain with the records the team creates.
*/
type Domain struct {
	models.HelpCenterDomain
	Target    string
	TXTName   string
	TXTValue  string
	CNAMEName string
}

/*
dress adds the records to create to a stored domain.
*/
func (d *Domains) dress(row models.HelpCenterDomain) Domain {
	return Domain{
		HelpCenterDomain: row, Target: d.Target, CNAMEName: row.Hostname,
		TXTName: domainRecordPrefix + row.Hostname, TXTValue: domainValuePrefix + row.VerificationToken,
	}
}

/*
Show returns the help center's domain, and false when it has none.
*/
func (d *Domains) Show(ctx context.Context, m authz.Membership, helpCenter ids.UUID) (Domain, bool, error) {
	if !m.Can("articles:read") && !m.Can("help_centers:read_settings") {
		return Domain{}, false, exceptions.ErrForbidden
	}
	var out Domain
	var found bool
	err := d.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := findHelpCenter(ctx, q, m.Workspace.ID, helpCenter); err != nil {
			return err
		}
		row, err := q.FindHelpCenterDomain(ctx, models.FindHelpCenterDomainParams{
			WorkspaceID: m.Workspace.ID, HelpCenterID: helpCenter,
		})
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("find help center domain: %w", err)
		}
		out, found = d.dress(row), true
		return nil
	})
	return out, found, err
}

/*
Attach claims a domain for the help center and makes it due a check at
once.
*/
func (d *Domains) Attach(ctx context.Context, m authz.Membership, helpCenter ids.UUID, hostname string,
) (Domain, error) {
	if !m.Can("help_centers:manage") {
		return Domain{}, exceptions.ErrForbidden
	}
	host, err := d.checkHost(hostname)
	if err != nil {
		return Domain{}, err
	}
	var out Domain
	err = d.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := findHelpCenter(ctx, q, m.Workspace.ID, helpCenter); err != nil {
			return err
		}
		_, err := q.FindHelpCenterDomain(ctx, models.FindHelpCenterDomainParams{
			WorkspaceID: m.Workspace.ID, HelpCenterID: helpCenter,
		})
		if err == nil {
			return ErrDomainExists
		}
		if !errors.Is(err, database.ErrNotFound) {
			return fmt.Errorf("find help center domain: %w", err)
		}
		taken, err := q.HelpCenterDomainTaken(ctx, host)
		if err != nil {
			return fmt.Errorf("check domain: %w", err)
		}
		if taken {
			return ErrDomainTaken
		}
		now := d.Clock.Now()
		row, err := q.CreateHelpCenterDomain(ctx, models.CreateHelpCenterDomainParams{
			ID: ids.NewV7At(now), WorkspaceID: m.Workspace.ID, HelpCenterID: helpCenter, Hostname: host,
			VerificationToken: strings.ToLower(str.Random(32)), Now: now,
		})
		if err != nil {
			return fmt.Errorf("create help center domain: %w", err)
		}
		out = d.dress(row)
		return nil
	})
	return out, err
}

/*
Recheck starts the domain's checks over, due at once, for a team that
just fixed its records.
*/
func (d *Domains) Recheck(ctx context.Context, m authz.Membership, helpCenter ids.UUID) (Domain, error) {
	if !m.Can("help_centers:manage") {
		return Domain{}, exceptions.ErrForbidden
	}
	var out Domain
	err := d.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		row, err := q.FindHelpCenterDomain(ctx, models.FindHelpCenterDomainParams{
			WorkspaceID: m.Workspace.ID, HelpCenterID: helpCenter,
		})
		if errors.Is(err, database.ErrNotFound) {
			return ErrDomainNotFound
		}
		if err != nil {
			return fmt.Errorf("find help center domain: %w", err)
		}
		row, err = q.RestartHelpCenterDomainChecks(ctx, models.RestartHelpCenterDomainChecksParams{
			Now: d.Clock.Now(), WorkspaceID: m.Workspace.ID, ID: row.ID,
		})
		if err != nil {
			return fmt.Errorf("restart domain checks: %w", err)
		}
		out = d.dress(row)
		return nil
	})
	return out, err
}

/*
Remove forgets the help center's domain; the help center answers only
at its own address again.
*/
func (d *Domains) Remove(ctx context.Context, m authz.Membership, helpCenter ids.UUID) error {
	if !m.Can("help_centers:manage") {
		return exceptions.ErrForbidden
	}
	var host string
	err := d.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		row, err := q.FindHelpCenterDomain(ctx, models.FindHelpCenterDomainParams{
			WorkspaceID: m.Workspace.ID, HelpCenterID: helpCenter,
		})
		if errors.Is(err, database.ErrNotFound) {
			return ErrDomainNotFound
		}
		if err != nil {
			return fmt.Errorf("find help center domain: %w", err)
		}
		host = row.Hostname
		return forgetDomain(ctx, q, models.SetHelpCenterCustomDomainParams{
			Now: d.Clock.Now(), WorkspaceID: m.Workspace.ID, ID: helpCenter,
		})
	})
	if err == nil && d.TLS != nil {
		_ = d.TLS.Remove(ctx, host)
	}
	return err
}

/*
checkHost normalizes a hostname, pasted with its scheme or not, and
refuses one that is malformed, a bare registrable domain, or a host of
the install itself.
*/
func (d *Domains) checkHost(raw string) (string, error) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	raw = strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(raw, "https://"), "http://"), "/")
	host, err := customdomain.Normalize(raw)
	if err != nil {
		return "", ErrDomainInvalid
	}
	if customdomain.IsApex(host) {
		return "", ErrDomainApex
	}
	for _, own := range d.Reserved {
		if own != "" && (host == own || strings.HasSuffix(host, "."+own)) {
			return "", ErrDomainReserved
		}
	}
	return host, nil
}

/*
DomainProbePath is where the routing probe answers on every host, so a
check can tell a domain reaches this install even behind a proxy.
*/
const DomainProbePath = "/.well-known/dimmi-domain"
