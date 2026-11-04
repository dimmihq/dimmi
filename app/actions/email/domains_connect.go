/*
|--------------------------------------------------------------------------
| Automatic DNS Setup
|--------------------------------------------------------------------------
|
| Where the domain's DNS host knows our Domain Connect template, the
| customer can have the host add every record in one approval instead of
| copying them. Our template carries SES records, so only an install that
| sends through SES offers it, and only once the operator turned it on
| with the signing key. What the host finds is cached for an hour, and
| every failure simply hides the button: copying the records by hand
| always works. The host sends the customer back to the domain's page,
| which then checks the records. Nothing trusts the host's result; the next check reads
| the records like any others.
|
*/

package email

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/customdomain"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/dns/domainconnect"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/integrations/email/dns"
	"github.com/dimmihq/dimmi/app/models"
)

/*
setupCacheTTL is how long what a host offers is remembered.
*/
const setupCacheTTL = time.Hour

/*
DNSSetups finds and starts automatic DNS setup. HTTP reaches the hosts,
Cache remembers what they offer, and Base is the app's own address the
host sends the customer back to.
*/
type DNSSetups struct {
	*Domains
	HTTP  *httpclient.Client
	Cache *cache.Store
	Base  string
}

/*
DNSSetup is a host that can set the domain up: its name and, once
started, the link to approve it there.
*/
type DNSSetup struct {
	HostName string
	ApplyURL string
}

/*
offered reports whether the install offers automatic setup at all.
*/
func (s *DNSSetups) offered(row models.EmailDomain) bool {
	connect := s.Config.Sending.DomainConnect

	return connect.Enabled && connect.PrivateKey != "" && row.Driver == dns.DriverSES && len(row.DkimTokens) == 3
}

/*
Offer returns the host that can set the domain up, if any.
*/
func (s *DNSSetups) Offer(ctx context.Context, row models.EmailDomain) (DNSSetup, bool) {
	if !s.offered(row) {
		return DNSSetup{}, false
	}

	host, ok := s.discover(ctx, zoneRoot(row.Domain))

	return DNSSetup{HostName: host.Name}, ok
}

/*
discover asks the domain's host, through the cache.
*/
func (s *DNSSetups) discover(ctx context.Context, root string) (domainconnect.Host, bool) {
	key := "email:domain_connect:" + root

	if cached, err := s.Cache.Get(ctx, key); err == nil {
		var host domainconnect.Host
		if json.Unmarshal(cached, &host) == nil {
			return host, host.SyncUX != ""
		}
	}

	finder := domainconnect.Finder{Resolver: s.resolvers()[0], Client: s.HTTP}
	host, _ := finder.Discover(ctx, root, s.template())

	if encoded, err := json.Marshal(host); err == nil {
		_ = s.Cache.Set(ctx, key, encoded, setupCacheTTL)
	}

	return host, host.SyncUX != ""
}

/*
template names our published template.
*/
func (s *DNSSetups) template() domainconnect.Template {
	connect := s.Config.Sending.DomainConnect

	return domainconnect.Template{ProviderID: connect.ProviderID, ServiceID: connect.ServiceID}
}

/*
Start returns the signed link to approve the setup at the host, and
notes that it was started.

Rules:
  - The caller needs email:manage.
  - The install offers it and the host knows the template.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrDNSSetupUnavailable], or a wrapped error.
*/
func (s *DNSSetups) Start(ctx context.Context, m authz.Membership, id ids.UUID) (DNSSetup, error) {
	if !m.Can("email:manage") {
		return DNSSetup{}, exceptions.ErrForbidden
	}

	row, err := domainIn(ctx, s.DB, m.Workspace.ID, id)
	if err != nil {
		return DNSSetup{}, err
	}

	root := zoneRoot(row.Domain)
	host, ok := s.discover(ctx, root)

	if !s.offered(row) || !ok {
		return DNSSetup{}, exceptions.ErrDNSSetupUnavailable
	}

	link, err := domainconnect.ApplyURL(host, s.template(), s.apply(row, root))
	if err != nil {
		return DNSSetup{}, fmt.Errorf("sign dns setup: %w", err)
	}

	err = s.DB.Transaction(inWorkspace(ctx, row.WorkspaceID), func(ctx context.Context, tx *database.Tx) error {
		return models.New(tx).MarkEmailDomainConnectApplied(ctx, models.MarkEmailDomainConnectAppliedParams{
			WorkspaceID: row.WorkspaceID, ID: row.ID, Now: s.Clock.Now(),
		})
	})
	if err != nil {
		return DNSSetup{}, fmt.Errorf("note dns setup: %w", err)
	}

	return DNSSetup{HostName: host.Name, ApplyURL: link}, nil
}

/*
apply fills the template's variables with the domain's own values.
*/
func (s *DNSSetups) apply(row models.EmailDomain, root string) domainconnect.Apply {
	subdomain := strings.TrimSuffix(strings.TrimSuffix(row.Domain, root), ".")
	mailFrom := s.Config.Sending.SES.MailFromSubdomain

	if row.MailFromDomain != nil {
		mailFrom = strings.TrimSuffix(*row.MailFromDomain, "."+row.Domain)
	}

	return domainconnect.Apply{
		Domain: root, Subdomain: subdomain,
		Variables: map[string]string{
			"dkim1": row.DkimTokens[0], "dkim2": row.DkimTokens[1], "dkim3": row.DkimTokens[2],
			"mailfrom": mailFrom, "token": row.OwnershipToken, "region": s.Region,
		},
		RedirectURI: strings.TrimSuffix(s.Base, "/") + "/d/settings/email/domains/" +
			models.PublicID(models.PrefixEmailDomain, row.ID) + "?dns_setup=applied",
		PrivateKey: s.Config.Sending.DomainConnect.PrivateKey,
		KeyHost:    s.Config.Sending.DomainConnect.KeyHost,
	}
}

/*
zoneRoot returns the registrable domain a name sits under, the zone its
DNS host knows.
*/
func zoneRoot(domain string) string {
	for name := domain; strings.Contains(name, "."); name = name[strings.Index(name, ".")+1:] {
		if customdomain.IsApex(name) {
			return name
		}
	}

	return domain
}
