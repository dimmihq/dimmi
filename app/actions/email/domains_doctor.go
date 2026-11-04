/*
|--------------------------------------------------------------------------
| Email Doctor
|--------------------------------------------------------------------------
|
| What an operator runs when mail from a domain misbehaves: one report in
| plain words on the customer-stream mailer, the inbound domain and the
| domain's own records. It reads live DNS through the same checker the
| domain pages use, so it says exactly what a customer would be told. It
| changes nothing.
|
*/

package email

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/i18n"

	"github.com/dimmihq/dimmi/app/integrations/email/dns"
	"github.com/dimmihq/dimmi/app/models"
)

/*
How serious a finding is.
*/
const (
	FindingOK   = "ok"
	FindingInfo = "info"
	FindingWarn = "warn"
	FindingFail = "fail"
)

/*
Finding is one line of the report.
*/
type Finding struct {
	Level   string
	Message string
}

/*
DiagnoseEmailDomain reports on the install's sending and the domain's
records, in the words of Lang. Production says whether the install runs
in production.

Errors: [exceptions.ErrInvalidDomain], or a wrapped database error.
*/
type DiagnoseEmailDomain struct {
	*Domains
	Lang       *i18n.Translator
	Production bool
}

/*
Handle returns the findings for the domain, mailer first.
*/
func (a *DiagnoseEmailDomain) Handle(ctx context.Context, domain string) ([]Finding, error) {
	name, err := a.normalizeDomain(domain)
	if err != nil {
		return nil, err
	}

	findings := a.mailer()
	findings = append(findings, a.inbound(ctx)...)

	claims, err := models.New(a.DB).ListEmailDomainClaims(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("list email domain claims: %w", err)
	}

	return append(findings, a.records(ctx, name, claims)...), nil
}

/*
mailer checks the customer-stream mailer: a driver that sends, its
credentials, and the webhook that reports bounces back.
*/
func (a *DiagnoseEmailDomain) mailer() []Finding {
	driver := a.Mailer.String("driver")

	switch driver {
	case "", "log":
		level := FindingWarn
		if a.Production {
			level = FindingFail
		}

		return []Finding{{level, "Customer mail is only written to the log. Set mail.mailers.customer to a provider."}}
	case dns.DriverSMTP:
		return a.relay()
	}

	findings := []Finding{{FindingOK, "Customer mail leaves through " + driver + "."}}
	if missing := a.missingCredential(driver); missing != "" {
		findings = append(findings, Finding{FindingFail, "The customer mailer has no " + missing + "."})
	}

	if !a.reportsEvents(driver) {
		findings = append(findings, Finding{
			FindingWarn,
			"No event webhook is set for " + driver + ", so bounces and complaints are not recorded.",
		})
	}

	return findings
}

/*
relay checks an SMTP relay, which must be a provider that accepts mail
with a login, never a receiving server reached directly.
*/
func (a *DiagnoseEmailDomain) relay() []Finding {
	findings := []Finding{{FindingOK, "Customer mail leaves through the SMTP relay " + a.Mailer.String("host") + "."}}

	if a.Mailer.String("port") == "25" && a.Mailer.String("username") == "" {
		findings = append(findings, Finding{
			FindingWarn,
			"The relay is reached on port 25 without a login, which looks like sending straight to " +
				"recipients' servers. Most of that mail is refused; use a provider's relay with a login.",
		})
	}

	if a.Config.Sending.RelaySPFInclude == "" {
		findings = append(findings, Finding{
			FindingInfo,
			"No SPF include is set for the relay, so domains are not told which one to add.",
		})
	}

	return findings
}

/*
missingCredential names the credential the driver lacks, empty when it
has what it needs.
*/
func (a *DiagnoseEmailDomain) missingCredential(driver string) string {
	key := map[string]string{
		dns.DriverSES: "region", dns.DriverPostmark: "token", dns.DriverResend: "api_key",
		dns.DriverMailgun: "api_key", dns.DriverSendGrid: "api_key",
	}[driver]

	if key == "" || a.Mailer.String(key) != "" {
		return ""
	}

	return key
}

/*
reportsEvents reports whether the provider's delivery events reach us.
*/
func (a *DiagnoseEmailDomain) reportsEvents(driver string) bool {
	sending := a.Config.Sending

	switch driver {
	case dns.DriverSES:
		return sending.SES.EventsTopicARN != ""
	case dns.DriverPostmark:
		return sending.Postmark.WebhookSecret != ""
	case dns.DriverResend:
		return sending.Resend.WebhookSecret != ""
	case dns.DriverMailgun:
		return sending.Mailgun.WebhookSecret != ""
	}

	return sending.SendGrid.WebhookSecret != ""
}

/*
inbound checks that mail for the inbound domain reaches a receiving
provider.
*/
func (a *DiagnoseEmailDomain) inbound(ctx context.Context) []Finding {
	findings := []Finding{{
		FindingInfo,
		"dimmi never runs a mail server of its own: mail arrives through your provider's inbound service.",
	}}

	domain := a.Config.InboundDomain
	if domain == "" {
		return append(findings, Finding{FindingInfo, "No inbound domain is set, so no forwarding addresses exist."})
	}

	exchangers, err := a.resolvers()[0].MX(ctx, domain)
	if err != nil || len(exchangers) == 0 {
		return append(findings, Finding{FindingFail, domain + " has no MX record, so forwarded mail never arrives."})
	}

	hosts := make([]string, 0, len(exchangers))
	for _, mx := range exchangers {
		hosts = append(hosts, mx.Host)
	}

	return append(findings, Finding{FindingOK, domain + " receives mail at " + strings.Join(hosts, ", ") + "."})
}

/*
records checks the domain's own records: who claims it, and the SPF and
DMARC mistakes that break mail whoever sends.
*/
func (a *DiagnoseEmailDomain) records(
	ctx context.Context, name string, claims []models.ListEmailDomainClaimsRow,
) []Finding {
	findings := []Finding{a.claimsFinding(name, claims)}
	spec := dns.Spec{Domain: name, Driver: a.Driver, SPFInclude: a.Config.Sending.RelaySPFInclude}
	expected := dns.Expected(spec)
	checker := dns.Checker{Resolvers: a.resolvers()}

	result := checker.Check(ctx, spec, expected[1:], nil)
	for _, record := range result.Records {
		if record.Status == dns.StatusInfo && record.Observed == "" && record.Selector != "" {
			findings = append(findings, Finding{
				FindingInfo,
				"No signing record shows at " + record.Host + "; check the selector in your provider's dashboard.",
			})
		}
	}

	for _, warning := range result.Warnings {
		message := a.Lang.T(ctx, "email_domains.warnings."+warning.Code, i18n.Args{
			"host": warning.Host, "value": warning.Value,
		})
		findings = append(findings, Finding{FindingWarn, message})
	}

	if len(result.Warnings) == 0 {
		findings = append(findings, Finding{FindingOK, "SPF and DMARC of " + name + " are in order."})
	}

	return findings
}

/*
claimsFinding says which workspaces added the domain and how far each
got, without naming them.
*/
func (a *DiagnoseEmailDomain) claimsFinding(name string, claims []models.ListEmailDomainClaimsRow) Finding {
	if len(claims) == 0 {
		return Finding{FindingInfo, "No workspace has added " + name + " yet."}
	}

	statuses := make([]string, 0, len(claims))
	for _, claim := range claims {
		statuses = append(statuses, claim.Status)
	}

	slices.Sort(statuses)

	return Finding{FindingInfo, fmt.Sprintf("%d workspace(s) added %s: %s.", len(claims), name,
		strings.Join(statuses, ", "))}
}
