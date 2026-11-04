/*
|--------------------------------------------------------------------------
| Email Domain Resource
|--------------------------------------------------------------------------
|
| A sending domain as the settings page shows it: every record with the
| name in the form the customer's DNS host wants typed, what the last
| check saw, and each mistake explained in plain words. Raw values found
| in DNS stay in observed and are never put into a sentence, so the
| explanation reads the same whatever the host returned.
|
*/

package resources

import (
	"context"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/mail"

	"github.com/dimmihq/dimmi/app/actions/email"
	"github.com/dimmihq/dimmi/app/integrations/email/dns"
	"github.com/dimmihq/dimmi/app/models"
)

/*
EmailDomain is one sending domain. Domain is how people write it and
ASCIIDomain how DNS stores it. HostGuide is the DNS host's instructions,
Detected saying whether it was read from the name servers.
*/
type EmailDomain struct {
	ID              string           `json:"id"`
	Object          string           `json:"object"`
	Domain          string           `json:"domain"`
	ASCIIDomain     string           `json:"ascii_domain"`
	Status          string           `json:"status"`
	Driver          string           `json:"driver"`
	Records         []DomainRecord   `json:"records"`
	Warnings        []DomainWarning  `json:"warnings"`
	ProviderHint    string           `json:"provider_hint"`
	HostGuide       HostGuide        `json:"host_guide"`
	MailFromDomain  *string          `json:"mail_from_domain"`
	ProviderTrusted bool             `json:"provider_trusted"`
	FailureReason   *string          `json:"failure_reason"`
	FailureMessage  *string          `json:"failure_message"`
	LastCheckedAt   *time.Time       `json:"last_checked_at"`
	NextCheckAt     *time.Time       `json:"next_check_at"`
	VerifiedAt      *time.Time       `json:"verified_at"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
	Links           EmailDomainLinks `json:"links"`
}

/*
DomainRecord is one record to publish. Name is fully qualified, Host is
what to type at the customer's DNS host.
*/
type DomainRecord struct {
	Purpose  string   `json:"purpose"`
	Type     string   `json:"type"`
	Name     string   `json:"name"`
	Host     string   `json:"host"`
	Value    string   `json:"value"`
	Priority int      `json:"priority,omitempty"`
	Required bool     `json:"required"`
	Status   string   `json:"status"`
	Observed []string `json:"observed"`
	Selector string   `json:"selector,omitempty"`
}

/*
DomainWarning is one mistake found, with what to do about it.
*/
type DomainWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Host    string `json:"host,omitempty"`
	Value   string `json:"value,omitempty"`
}

/*
HostGuide is a DNS host's instructions, its steps and quirks in words.
*/
type HostGuide struct {
	Key      string   `json:"key"`
	Name     string   `json:"name"`
	DeepLink string   `json:"deep_link"`
	HostForm string   `json:"host_form"`
	Steps    []string `json:"steps"`
	Quirks   []string `json:"quirks"`
	Detected bool     `json:"detected"`
}

/*
EmailDomainLinks are where the domain, its checks and its zone file are.
*/
type EmailDomainLinks struct {
	Self     string `json:"self"`
	Checks   string `json:"checks"`
	ZoneFile string `json:"zone_file"`
}

/*
NewEmailDomain returns the domain as the API shows it. Hint is the DNS
host read from the name servers, empty when not looked up; base is the
API's address.
*/
func NewEmailDomain(ctx context.Context, row models.EmailDomain, hint, base string) EmailDomain {
	id := models.PublicID(models.PrefixEmailDomain, row.ID)
	self := strings.TrimSuffix(base, "/") + "/api/v1/email-domains/" + id
	guide := dns.HostByKey(dns.Hosts(), hint)
	detected := hint != "" && hint != dns.HostOther

	if row.DnsHostOverride != nil {
		guide, detected = dns.HostByKey(dns.Hosts(), *row.DnsHostOverride), false
	}

	out := EmailDomain{
		ID: id, Object: "email_domain", Domain: mail.UnicodeDomain(row.Domain), ASCIIDomain: row.Domain,
		Status: row.Status, Driver: row.Driver, ProviderHint: guide.Key, HostGuide: newHostGuide(ctx, guide, row),
		MailFromDomain: row.MailFromDomain, ProviderTrusted: row.ProviderTrustedAt != nil,
		FailureReason: row.FailureReason, LastCheckedAt: row.LastCheckedAt, NextCheckAt: row.NextCheckAt,
		VerifiedAt: row.VerifiedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		Links:    EmailDomainLinks{Self: self, Checks: self + "/checks", ZoneFile: self + "?format=bind"},
		Records:  newDomainRecords(email.Records(row), guide, row.Domain),
		Warnings: newDomainWarnings(ctx, email.Warnings(row)),
	}

	out.HostGuide.Detected = detected

	if row.FailureReason != nil {
		message := i18n.T(ctx, "email_domains.failures."+*row.FailureReason, nil)
		out.FailureMessage = &message
	}

	return out
}

/*
NewEmailDomains returns the domains as the API lists them.
*/
func NewEmailDomains(ctx context.Context, rows []models.EmailDomain, base string) []EmailDomain {
	out := make([]EmailDomain, 0, len(rows))
	for _, row := range rows {
		out = append(out, NewEmailDomain(ctx, row, "", base))
	}

	return out
}

/*
newDomainRecords writes the records with names as the host wants them.
*/
func newDomainRecords(records []dns.Record, host dns.Host, domain string) []DomainRecord {
	out := make([]DomainRecord, 0, len(records))
	for _, r := range records {
		observed := []string{}
		if r.Observed != "" {
			observed = strings.Split(r.Observed, " | ")
		}

		out = append(out, DomainRecord{
			Purpose: r.Purpose, Type: r.Type, Name: r.Host, Host: dns.HostedName(host, r.Host, domain), Value: r.Value,
			Priority: r.Priority, Required: r.Required, Status: r.Status, Observed: observed, Selector: r.Selector,
		})
	}

	return out
}

/*
newDomainWarnings explains each mistake in words.
*/
func newDomainWarnings(ctx context.Context, warnings []dns.Warning) []DomainWarning {
	out := make([]DomainWarning, 0, len(warnings))
	for _, w := range warnings {
		out = append(out, DomainWarning{
			Code: w.Code, Host: w.Host, Value: w.Value,
			Message: i18n.T(ctx, "email_domains.warnings."+w.Code, i18n.Args{"host": w.Host, "value": w.Value}),
		})
	}

	return out
}

/*
newHostGuide writes a host's steps and quirks in words.
*/
func newHostGuide(ctx context.Context, host dns.Host, row models.EmailDomain) HostGuide {
	guide := HostGuide{
		Key: host.Key, Name: host.Name, DeepLink: dns.DeepLink(host, row.Domain), HostForm: host.HostForm,
		Steps: make([]string, 0, len(host.Steps)), Quirks: make([]string, 0, len(host.Quirks)),
	}

	for _, key := range host.Steps {
		guide.Steps = append(guide.Steps, i18n.T(ctx, key, nil))
	}

	for _, key := range host.Quirks {
		guide.Quirks = append(guide.Quirks, i18n.T(ctx, key, nil))
	}

	return guide
}
