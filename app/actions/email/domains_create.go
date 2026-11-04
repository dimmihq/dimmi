/*
|--------------------------------------------------------------------------
| Adding a Sending Domain
|--------------------------------------------------------------------------
|
| Adding a domain registers it with whoever signs the mail and answers
| with every record to publish, before anything is checked. SES and a
| provider's domain API give their own signing records; a relay we sign
| for gets a fresh key of its own, kept encrypted under the workspace's
| keys. Several workspaces may add the same domain while it is unproven;
| only one can ever verify it.
|
*/

package email

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/mail/dkim"
	"github.com/dimmihq/dimmi/framework/support/str"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/integrations/email/dns"
	"github.com/dimmihq/dimmi/app/models"
)

/*
domainUnique is the constraint that keeps one live row per domain in a
workspace.
*/
const domainUnique = "email_domains_workspace_id_domain_key"

/*
sendingDrivers are the drivers a domain can be set up for.
*/
func sendingDrivers() []string {
	return []string{
		dns.DriverSES, dns.DriverSMTP, dns.DriverPostmark, dns.DriverResend, dns.DriverMailgun, dns.DriverSendGrid,
	}
}

/*
CreateEmailDomainInput is who asks, the domain, and the return path
subdomain, bounce when empty.
*/
type CreateEmailDomainInput struct {
	Membership authz.Membership
	Domain     string
	MailFrom   string
}

/*
CreateEmailDomain adds a sending domain as pending.

Rules:
  - The caller needs email:manage.
  - The domain must be one a customer can own: see normalizeDomain.
  - A workspace adds a domain once.
  - The install must send customer mail through a provider.

Errors: [exceptions.ErrForbidden], [exceptions.ErrSendingNotConfigured],
[exceptions.ErrInvalidDomain],
[exceptions.ErrPublicMailboxDomain], [exceptions.ErrDomainExists],
[exceptions.ErrEmailProviderUnavailable], or a wrapped database error.
*/
type CreateEmailDomain struct {
	*Domains
}

/*
Handle registers the domain and returns its row.
*/
func (a *CreateEmailDomain) Handle(
	ctx context.Context, in CreateEmailDomainInput,
) (models.EmailDomain, error) {
	if !in.Membership.Can("email:manage") {
		return models.EmailDomain{}, exceptions.ErrForbidden
	}

	if !slices.Contains(sendingDrivers(), a.Driver) {
		return models.EmailDomain{}, exceptions.ErrSendingNotConfigured
	}

	name, err := a.normalizeDomain(in.Domain)
	if err != nil {
		return models.EmailDomain{}, err
	}

	ws := in.Membership.Workspace.ID
	if err := a.refuseDuplicate(ctx, ws, name); err != nil {
		return models.EmailDomain{}, err
	}

	params, err := a.provision(ctx, ws, name, in.MailFrom)
	if err != nil {
		return models.EmailDomain{}, err
	}

	params.CreatedByMemberID = &in.Membership.Member.ID

	var row models.EmailDomain

	err = a.DB.Transaction(inWorkspace(ctx, ws), func(ctx context.Context, tx *database.Tx) error {
		row, err = models.New(tx).CreateEmailDomain(ctx, params)

		return err
	})
	if database.IsUniqueViolation(err, domainUnique) {
		return models.EmailDomain{}, exceptions.ErrDomainExists
	}

	if err != nil {
		return models.EmailDomain{}, fmt.Errorf("create email domain: %w", err)
	}

	return row, nil
}

/*
refuseDuplicate refuses a domain the workspace already added, before the
provider is asked to set it up again.
*/
func (a *CreateEmailDomain) refuseDuplicate(ctx context.Context, ws ids.UUID, name string) error {
	return a.DB.Transaction(inWorkspace(ctx, ws), func(ctx context.Context, tx *database.Tx) error {
		_, err := models.New(tx).FindEmailDomainByName(ctx, models.FindEmailDomainByNameParams{
			WorkspaceID: ws, Domain: name,
		})
		if err == nil {
			return exceptions.ErrDomainExists
		}

		if !isNotFound(err) {
			return fmt.Errorf("find email domain: %w", err)
		}

		return nil
	})
}

/*
provision sets the domain up for the install's driver and returns the
row to insert, its records included.
*/
func (a *CreateEmailDomain) provision(
	ctx context.Context, ws ids.UUID, name, mailFrom string,
) (models.CreateEmailDomainParams, error) {
	now := a.Clock.Now()
	token := ownershipToken()

	got, err := a.setUp(ctx, models.EmailDomain{WorkspaceID: ws, Domain: name, OwnershipToken: token}, mailFrom)
	if err != nil {
		return models.CreateEmailDomainParams{}, err
	}

	return models.CreateEmailDomainParams{
		WorkspaceID: ws, Domain: name, Driver: a.Driver, OwnershipToken: token, Now: now,
		DkimSelector: got.DkimSelector, DkimPrivateKeyEncrypted: got.DkimPrivateKeyEncrypted,
		DkimTokens: got.DkimTokens, DkimSigningHostedZone: got.DkimSigningHostedZone,
		MailFromDomain: got.MailFromDomain, ProviderDomainID: got.ProviderDomainID,
		ProviderTrustedAt: got.ProviderTrustedAt, Records: got.Records,
	}, nil
}

/*
setUp registers the row's domain for the install's driver and returns the
row as it should then be stored: its signing material and the records it
needs. The return path goes on the subdomain, bounce when empty.
*/
func (d *Domains) setUp(ctx context.Context, row models.EmailDomain, subdomain string) (models.EmailDomain, error) {
	row.Driver, row.ProviderTrustedAt = d.Driver, nil
	row.DkimSelector, row.DkimPrivateKeyEncrypted, row.DkimTokens = nil, nil, nil
	row.DkimSigningHostedZone, row.MailFromDomain, row.ProviderDomainID = nil, nil, nil
	publicKey := ""

	var provided []mail.DomainRecord

	switch {
	case d.Driver == dns.DriverSMTP:
		key, err := d.newRelayKey(ctx, row.WorkspaceID, row.Domain, d.Clock.Now().Format("200601"))
		if err != nil {
			return row, err
		}

		row.DkimSelector, row.DkimPrivateKeyEncrypted, publicKey = &key.selector, key.sealed, key.public
	case d.Provider != nil:
		got, err := d.Provider.Provision(ctx, row.WorkspaceID, row.Domain,
			cmpOr(subdomain, d.Config.Sending.SES.MailFromSubdomain))
		if err != nil {
			return row, providerError(err)
		}

		row.DkimTokens, row.DkimSigningHostedZone = got.Tokens, optional(got.Zone)
		row.MailFromDomain, row.ProviderDomainID = optional(got.MailFrom), optional(got.ProviderID)
		provided = got.Records
	}

	if manual(d.Driver) && d.trusted(row.Domain) {
		now := d.Clock.Now()
		row.ProviderTrustedAt = &now
	}

	row.Records = encode(d.expected(row, publicKey, provided))

	return row, nil
}

/*
cmpOr returns the first non-empty text.
*/
func cmpOr(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}

	return ""
}

/*
providerError tells the caller the provider did not answer, keeping the
cause for the log.
*/
func providerError(err error) error {
	return fmt.Errorf("%w: %w", exceptions.ErrEmailProviderUnavailable, err)
}

/*
expected returns the records a row needs: our own for SES and a relay,
the provider's when it reported them, and guidance otherwise.
*/
func (d *Domains) expected(row models.EmailDomain, publicKey string, provider []mail.DomainRecord) []dns.Record {
	base := dns.Expected(d.spec(row, publicKey))
	if len(provider) == 0 {
		return base
	}

	out := []dns.Record{base[0]}
	for _, record := range provider {
		out = append(out, providerRecord(record))
	}

	return append(out, base[len(base)-1])
}

/*
relayKey is a fresh signing key: its selector, its public half for the
record, and its private half sealed for storage.
*/
type relayKey struct {
	selector string
	public   string
	sealed   encryption.Sealed
}

/*
newRelayKey makes a signing key for a relay's mail, its selector named
after the month it was made in.
*/
func (d *Domains) newRelayKey(ctx context.Context, ws ids.UUID, domain, month string) (relayKey, error) {
	key, err := dkim.GenerateKey()
	if err != nil {
		return relayKey{}, fmt.Errorf("generate signing key: %w", err)
	}

	selector := d.Config.Sending.SelectorPrefix + month + strings.ToLower(str.Random(2))

	sealed, err := encryption.Seal(ctx, d.Keys, keySlot(ws, domain, selector), key.PrivatePEM)
	if err != nil {
		return relayKey{}, fmt.Errorf("seal signing key: %w", err)
	}

	return relayKey{selector: selector, public: key.Public, sealed: sealed}, nil
}

/*
keySlot is where a domain's signing key is sealed: the workspace's keys,
bound to the domain and selector.
*/
func keySlot(ws ids.UUID, domain, selector string) encryption.Slot {
	return encryption.Slot{
		Scope: encryption.Scope{Kind: "workspace", ID: ws.String()},
		AAD:   []byte("email_domains.dkim:" + domain + ":" + selector),
	}
}

/*
ownershipToken returns the value only this workspace's TXT record holds.
*/
func ownershipToken() string {
	return strings.ToLower(str.Random(24))
}
