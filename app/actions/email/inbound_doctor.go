/*
|--------------------------------------------------------------------------
| Inbound Doctor
|--------------------------------------------------------------------------
|
| An operator setting up a receiving provider has many ways to get it
| almost right, and mail that goes nowhere says nothing. These checks say
| in plain words what is wrong and which setting fixes it: the inbound
| domain's MX pointing at the wrong host, a source missing a key, an
| address the provider cannot reach, a setting the provider's deliveries
| say is off, or addresses that have gone quiet.
|
*/

package email

import (
	"context"
	"fmt"
	"net/netip"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/dns"
	"github.com/dimmihq/dimmi/framework/mailbox"

	"github.com/dimmihq/dimmi/app/models"
	appconfig "github.com/dimmihq/dimmi/config"
)

/*
CheckStatus says how a check went.
*/
type CheckStatus string

/*
The outcomes of a check.
*/
const (
	CheckPass CheckStatus = "pass"
	CheckWarn CheckStatus = "warn"
	CheckFail CheckStatus = "fail"
)

/*
quietAfter is how long without mail makes an install with addresses
worth a warning.
*/
const quietAfter = 7 * 24 * time.Hour

/*
maxClockSkew is how far this server's clock may sit from the database's
before signed webhooks are at risk.
*/
const maxClockSkew = time.Minute

/*
Check is one finding of the doctor.
*/
type Check struct {
	ID      string
	Status  CheckStatus
	Message string
}

/*
InboundDoctor checks how mail reaches the install.
*/
type InboundDoctor struct {
	DB      *database.DB
	Clock   clock.Clock
	DNS     dns.Resolver
	Sources *mailbox.Manager
	Health  *InboundHealth
	Config  appconfig.EmailConfig
	AppURL  string
}

/*
Handle runs every check.
*/
func (d *InboundDoctor) Handle(ctx context.Context) []Check {
	checks := []Check{d.mxCheck(ctx)}
	checks = append(checks, d.sourceChecks()...)
	checks = append(checks, d.urlCheck())
	checks = append(checks, d.signalChecks(ctx)...)
	checks = append(checks, d.clockCheck(ctx))

	return append(checks, d.recentCheck(ctx))
}

/*
mxHosts lists the hosts each source receives mail on. Resend shows its
own value per domain, so only an MX being there can be checked.
*/
func mxHosts(source string) []string {
	switch source {
	case appconfig.InboundSourceSES:
		return []string{"amazonaws.com"}
	case appconfig.InboundSourcePostmark:
		return []string{"inbound.postmarkapp.com"}
	case appconfig.InboundSourceMailgun:
		return []string{"mailgun.org"}
	case appconfig.InboundSourceSendGrid:
		return []string{"mx.sendgrid.net"}
	case appconfig.InboundSourceCloudflare:
		return []string{"mx.cloudflare.net"}
	default:
		return nil
	}
}

func (d *InboundDoctor) mxCheck(ctx context.Context) Check {
	const id = "email.inbound_mx"

	if !d.Config.Receives() {
		return Check{id, CheckPass, "No inbound domain is set, so only connected mailboxes receive mail."}
	}

	records, err := d.DNS.MX(ctx, d.Config.InboundDomain)
	if err != nil || len(records) == 0 {
		detail := fmt.Sprintf("%s has no MX record, so no mail can reach it.", d.Config.InboundDomain)

		return Check{id, CheckFail, detail}
	}

	for _, source := range d.Config.Inbound.Drivers {
		if source == appconfig.InboundSourceResend {
			return Check{id, CheckWarn, "An MX exists. Compare it with the value Resend shows for the domain."}
		}

		if mxMatches(records, mxHosts(source)) {
			return Check{id, CheckPass, fmt.Sprintf("%s receives through %s.", d.Config.InboundDomain, source)}
		}
	}

	return Check{id, CheckFail, fmt.Sprintf(
		"The MX of %s is %s, which is none of the enabled sources: %s.",
		d.Config.InboundDomain, records[0].Host, strings.Join(d.Config.Inbound.Drivers, ", "),
	)}
}

func mxMatches(records []dns.MX, hosts []string) bool {
	return slices.ContainsFunc(records, func(record dns.MX) bool {
		host := strings.TrimSuffix(strings.ToLower(record.Host), ".")

		return slices.ContainsFunc(hosts, func(suffix string) bool { return strings.HasSuffix(host, suffix) })
	})
}

func (d *InboundDoctor) sourceChecks() []Check {
	checks := make([]Check, 0, len(d.Config.Inbound.Drivers))

	for _, source := range d.Config.Inbound.Drivers {
		if source == appconfig.InboundSourceTest {
			continue
		}

		id := "email.inbound_source." + source
		if _, err := d.Sources.Receiver(source); err != nil {
			checks = append(checks, Check{id, CheckFail, err.Error()})

			continue
		}

		checks = append(checks, Check{id, CheckPass, "Configured."})
	}

	return checks
}

/*
urlCheck warns about an address a provider cannot post to.
*/
func (d *InboundDoctor) urlCheck() Check {
	const id = "email.inbound_url"

	parsed, err := url.Parse(d.AppURL)
	if err != nil || parsed.Scheme != "https" || privateHost(parsed.Hostname()) {
		return Check{id, CheckWarn, fmt.Sprintf(
			"app.url is %s. Providers post to it, so it must be public and use https.", d.AppURL,
		)}
	}

	return Check{id, CheckPass, "Providers can reach " + d.AppURL + "."}
}

func privateHost(host string) bool {
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}

	addr, err := netip.ParseAddr(host)

	return err == nil && (addr.IsPrivate() || addr.IsLoopback())
}

func (d *InboundDoctor) signalChecks(ctx context.Context) []Check {
	checks := make([]Check, 0, len(d.Config.Inbound.Drivers))

	for _, source := range d.Config.Inbound.Drivers {
		setting, err := d.Health.Misconfigured(ctx, source)
		if err != nil || setting == "" {
			continue
		}

		checks = append(checks, Check{"email.misconfigured." + source, CheckFail, fmt.Sprintf(
			"The last delivery from %s says this needs changing: %s.", source, setting,
		)})
	}

	return checks
}

/*
clockCheck compares this server's clock with the database's. Providers
sign each post with its time and a post over five minutes old is
refused, so a clock that drifted turns every delivery away.
*/
func (d *InboundDoctor) clockCheck(ctx context.Context) Check {
	const id = "email.clock"

	now, err := models.New(d.DB).DatabaseNow(ctx)
	if err != nil {
		return Check{id, CheckWarn, "Could not compare this server's clock with the database's."}
	}

	skew := d.Clock.Now().Sub(now).Abs()
	if skew > maxClockSkew {
		return Check{id, CheckWarn, fmt.Sprintf(
			"This server's clock is %s off the database's. Signed webhooks over 5 minutes old are refused.",
			skew.Round(time.Second),
		)}
	}

	return Check{id, CheckPass, "This server's clock agrees with the database's."}
}

func (d *InboundDoctor) recentCheck(ctx context.Context) Check {
	const id = "email.inbound_recent"

	q := models.New(d.DB)

	addresses, err := q.CountLiveEmailAddresses(ctx)
	if err != nil {
		return Check{id, CheckWarn, "Could not read when mail last arrived."}
	}

	latest, err := q.LatestInboundEmails(ctx)
	if err != nil {
		return Check{id, CheckWarn, "Could not read when mail last arrived."}
	}

	since := d.Clock.Now().Add(-quietAfter)
	if addresses > 0 && !slices.ContainsFunc(latest, func(row models.LatestInboundEmailsRow) bool {
		return row.LatestAt.After(since)
	}) {
		return Check{id, CheckWarn, fmt.Sprintf("%d addresses exist but no mail arrived in 7 days.", addresses)}
	}

	return Check{id, CheckPass, "Mail arrived in the last 7 days, or no address exists yet."}
}
