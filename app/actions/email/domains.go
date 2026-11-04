/*
|--------------------------------------------------------------------------
| Sending Domains
|--------------------------------------------------------------------------
|
| A workspace proves it owns a domain before any mail leaves as it. The
| proof is a TXT record only that workspace knows the value of, so one
| workspace on a shared install can never send as another's domain. On
| top of it the domain needs the signing records of whoever sends: SES,
| a relay we sign for, or a provider the operator set up.
|
| Everything a domain needs from outside, the provider's identity and
| the DNS host's answers, is reached through the dependencies held here,
| so the actions read the same in every edition and every test.
|
*/

package email

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/clock"
	fconfig "github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/customdomain"
	"github.com/dimmihq/dimmi/framework/database"
	fdns "github.com/dimmihq/dimmi/framework/dns"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/mail/dkim"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/integrations/email/dns"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/config"
)

/*
The states a sending domain moves through.
*/
const (
	DomainPending   = "pending"
	DomainVerifying = "verifying"
	DomainVerified  = "verified"
	DomainDegraded  = "degraded"
	DomainFailed    = "failed"
)

/*
The reasons a domain fails, each a language key under
email_domains.failures.
*/
const (
	FailureNotVerified         = "not_verified"
	FailureRecordsRemoved      = "records_removed"
	FailureOwnershipTransfered = "ownership_transferred"
	FailureProviderRejected    = "provider_rejected_sender"
)

/*
DomainProvider is the sending provider's side of a domain: SES's
identities, or a provider's domain API. Every method may be missing,
which the zero DomainProvider says: a provider set up by hand.
*/
type DomainProvider interface {
	/*
		Provision registers the domain at the provider, its return path on
		the subdomain, and returns what the records are built from.
	*/
	Provision(ctx context.Context, workspaceID ids.UUID, domain, mailFrom string) (Provisioned, error)

	/*
		Inspect returns what the provider found for the domain now.
	*/
	Inspect(ctx context.Context, row models.EmailDomain) (Inspection, error)

	/*
		Restart asks the provider to look for the signing records again.
	*/
	Restart(ctx context.Context, row models.EmailDomain) error

	/*
		Remove takes the domain off the provider, if it holds it.
	*/
	Remove(ctx context.Context, row models.EmailDomain) error
}

/*
Provisioned is what a provider returns for a new domain: SES's signing
tokens and their zone, the return path it now uses, or a provider's own
id with the records it asks for.
*/
type Provisioned struct {
	Tokens     []string
	Zone       string
	MailFrom   string
	ProviderID string
	Records    []mail.DomainRecord
}

/*
Inspection is what a provider says about a domain: whether it signs for
it, its own words for the state, and, for a provider with a domain API,
its records. Restartable says SES gave up looking for the signing
records, which it may be asked to do again.
*/
type Inspection struct {
	Signs       bool
	Status      string
	Records     []mail.DomainRecord
	Restartable bool
}

/*
Domains holds what every domain action needs. Driver is the install's
customer-stream driver at the time and Mailer that mailer's
configuration, Provider its side of a domain, nil for a provider set up
by hand, and Region the SES region the return path points at.
*/
type Domains struct {
	DB       *database.DB
	Mailer   fconfig.Section
	Clock    clock.Clock
	Events   *events.Dispatcher
	Keys     encryption.KeyManager
	DNS      *fdns.Client
	Provider DomainProvider
	Driver   string
	Region   string
	Config   config.EmailConfig
	Edition  config.Edition
}

/*
resolvers returns the servers a check asks: the configured ones, or the
install's own resolver.
*/
func (d *Domains) resolvers() []fdns.Resolver {
	if len(d.Config.Sending.Resolvers) == 0 {
		return []fdns.Resolver{d.DNS}
	}

	out := make([]fdns.Resolver, 0, len(d.Config.Sending.Resolvers))
	for _, server := range d.Config.Sending.Resolvers {
		out = append(out, d.DNS.Via(server))
	}

	return out
}

/*
HostHint returns the key of whoever hosts the domain's DNS, asking the
servers checks ask.
*/
func (d *Domains) HostHint(ctx context.Context, domain string) string {
	return dns.HostHint(ctx, d.resolvers()[0], dns.Hosts(), domain)
}

/*
publicMailboxDomains are the domains of free mailboxes, which nobody adds
as their own: forwarding a Gmail address is the way to use one.
*/
func publicMailboxDomains() []string {
	return []string{
		"gmail.com", "googlemail.com", "outlook.com", "hotmail.com", "live.com", "msn.com", "yahoo.com",
		"ymail.com", "icloud.com", "me.com", "mac.com", "aol.com", "proton.me", "protonmail.com", "pm.me",
		"gmx.com", "gmx.de", "web.de", "mail.com", "zoho.com", "yandex.com", "yandex.ru", "fastmail.com",
		"hey.com", "tutanota.com", "qq.com", "163.com",
	}
}

/*
normalizeDomain returns the ASCII form of a domain a workspace may add,
or exceptions.ErrInvalidDomain: an address, a public suffix, the
install's own mail domains, and free mailbox domains are refused.
*/
func (d *Domains) normalizeDomain(raw string) (string, error) {
	name, err := customdomain.Normalize(raw)
	if err != nil || len(name) > 253 {
		return "", exceptions.ErrInvalidDomain
	}

	reserved := []string{d.Config.InboundDomain, d.Config.DefaultSenderDomain}
	if d.Edition == config.EditionCloud {
		reserved = append(reserved, "trydimmi.com")
	}

	for _, own := range reserved {
		if own != "" && (name == own || strings.HasSuffix(name, "."+own)) {
			return "", exceptions.ErrInvalidDomain
		}
	}

	if slices.Contains(publicMailboxDomains(), name) {
		return "", exceptions.ErrPublicMailboxDomain
	}

	return name, nil
}

/*
manual reports whether the driver's domains are set up at the provider
rather than signed by us or by SES.
*/
func manual(driver string) bool {
	return driver != dns.DriverSES && driver != dns.DriverSMTP
}

/*
trusted reports whether the operator listed the domain, or a parent of
it, as set up at the provider.
*/
func (d *Domains) trusted(domain string) bool {
	return slices.ContainsFunc(d.Config.Sending.ProviderVerifiedDomains, func(listed string) bool {
		listed = strings.ToLower(strings.TrimSpace(listed))

		return listed != "" && (domain == listed || strings.HasSuffix(domain, "."+listed))
	})
}

/*
spec returns what the row's records are built from.
*/
func (d *Domains) spec(row models.EmailDomain, publicKey string) dns.Spec {
	spec := dns.Spec{
		Domain: row.Domain, OwnershipToken: row.OwnershipToken, Driver: row.Driver,
		Tokens: row.DkimTokens, PublicKey: publicKey, SPFInclude: d.Config.Sending.RelaySPFInclude,
	}

	if row.DkimSigningHostedZone != nil {
		spec.Zone = *row.DkimSigningHostedZone
	}

	if row.MailFromDomain != nil {
		spec.MailFrom = *row.MailFromDomain
	}

	if row.DkimSelector != nil {
		spec.Selector = *row.DkimSelector
	}

	spec.Region = d.Region

	return spec
}

/*
Warnings returns the stored warnings of the row.
*/
func Warnings(row models.EmailDomain) []dns.Warning {
	var out []dns.Warning
	if err := json.Unmarshal(row.Warnings, &out); err != nil {
		return nil
	}

	return out
}

/*
Records returns the stored records of the row.
*/
func Records(row models.EmailDomain) []dns.Record {
	var out []dns.Record
	if err := json.Unmarshal(row.Records, &out); err != nil {
		return nil
	}

	return out
}

/*
encode returns the value as JSON for a jsonb column.
*/
func encode(value any) json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage("[]")
	}

	return encoded
}

/*
findDomain returns a live domain of the workspace, exceptions.ErrNotFound
when there is none.
*/
func findDomain(ctx context.Context, q *models.Queries, workspaceID, id ids.UUID) (models.EmailDomain, error) {
	row, err := q.FindEmailDomain(ctx, models.FindEmailDomainParams{WorkspaceID: workspaceID, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return models.EmailDomain{}, exceptions.ErrNotFound
	}

	if err != nil {
		return models.EmailDomain{}, fmt.Errorf("find email domain: %w", err)
	}

	return row, nil
}

/*
isNotFound reports whether a query found no row.
*/
func isNotFound(err error) bool {
	return errors.Is(err, database.ErrNotFound)
}

/*
providerRecord turns a record a provider asked for into one of ours. Its
own check stands for the record's status, and records it needs to sign
or bounce are required.
*/
func providerRecord(r mail.DomainRecord) dns.Record {
	status := dns.StatusMissing
	if r.Valid {
		status = dns.StatusVerified
	}

	purpose := map[string]string{
		mail.PurposeDKIM: dns.PurposeDKIM, mail.PurposeReturnPath: dns.PurposeReturnPath,
		mail.PurposeSPF: dns.PurposeSPF, mail.PurposeMX: dns.PurposeReturnPath,
	}[r.Purpose]

	return dns.Record{
		Purpose: cmpOr(purpose, r.Purpose), Type: r.Type, Host: r.Host, Value: r.Value, Priority: r.Priority,
		Required: r.Purpose == mail.PurposeDKIM, Status: status,
	}
}

/*
publicKey returns the public half of a relay signing key of the row, by
its selector and sealed private half; empty when there is none.
*/
func (d *Domains) publicKey(
	ctx context.Context, row models.EmailDomain, selector *string, sealed encryption.Sealed,
) (string, error) {
	if selector == nil || len(sealed) == 0 {
		return "", nil
	}

	private, err := d.openKey(ctx, row, selector, sealed)
	if err != nil {
		return "", err
	}

	public, err := dkim.PublicKey(private)
	if err != nil {
		return "", fmt.Errorf("read signing key: %w", err)
	}

	return public, nil
}

/*
openKey returns the PEM private half of a relay signing key of the row.
*/
func (d *Domains) openKey(
	ctx context.Context, row models.EmailDomain, selector *string, sealed encryption.Sealed,
) (string, error) {
	private, err := encryption.Open[string](ctx, d.Keys, sealed, keySlot(row.WorkspaceID, row.Domain, *selector).AAD)
	if err != nil {
		return "", fmt.Errorf("open signing key: %w", err)
	}

	return private, nil
}
