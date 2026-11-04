/*
|--------------------------------------------------------------------------
| Mailgun Domains
|--------------------------------------------------------------------------
|
| Mailgun names a domain by its name, so the name is the id. It lists the
| records for sending and for receiving; only sending ones are reported,
| because receiving mail does not depend on this setup. The tracking
| CNAME it always lists is reported with its own purpose, so a caller can
| leave it out.
|
| Click and open tracking each take a separate call after creating the
| domain, on the older API version, as does deleting.
|
*/

package mailgun

import (
	"cmp"
	"context"
	"encoding/base64"
	"net/url"
	"strings"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/mail/internal/provider"
)

/*
Answers and content types of the API. Mailgun reports a domain and a
record it found in DNS as stateActive and recordValid. Its form requests
use formEncoded.
*/
const (
	stateActive = "active"
	recordValid = "valid"
	formEncoded = "application/x-www-form-urlencoded"
)

/*
Domains manages sending domains through the Mailgun API.
*/
type Domains struct {
	client   *httpclient.Client
	token    string
	endpoint string
}

/*
NewDomains returns a domain manager using the API key in the options.
Set the endpoint to https://api.eu.mailgun.net for the EU region.
*/
func NewDomains(client *httpclient.Client, opts mail.DomainOptions) *Domains {
	return &Domains{client: client, token: opts.Token, endpoint: cmp.Or(opts.Endpoint, "https://api.mailgun.net")}
}

/*
CreateDomain posts the domain and then turns its tracking off. When that
fails, the domain comes back with ErrDomainTrackingLeftOn.
*/
func (d *Domains) CreateDomain(ctx context.Context, domain string) (mail.ProviderDomain, error) {
	form := url.Values{"name": {domain}}.Encode()
	resp, err := d.request().WithHeader("Content-Type", formEncoded).Post(ctx, d.endpoint+"/v4/domains", form)

	created, err := parse("create domain", resp, err)
	if err != nil {
		return mail.ProviderDomain{}, err
	}

	if err := d.turnOffTracking(ctx, created.ID); err != nil {
		return created, provider.TrackingLeftOn("mailgun", err)
	}

	return created, nil
}

/*
GetDomain reads the domain.
*/
func (d *Domains) GetDomain(ctx context.Context, id string) (mail.ProviderDomain, error) {
	if id == "" {
		return mail.ProviderDomain{}, provider.DomainNotFound("mailgun", "get domain")
	}

	resp, err := d.request().Get(ctx, d.endpoint+"/v4/domains/"+url.PathEscape(id))

	return parse("get domain", resp, err)
}

/*
VerifyDomain asks Mailgun to look at DNS.
*/
func (d *Domains) VerifyDomain(ctx context.Context, id string) (mail.ProviderDomain, error) {
	if id == "" {
		return mail.ProviderDomain{}, provider.DomainNotFound("mailgun", "verify domain")
	}

	resp, err := d.request().Put(ctx, d.endpoint+"/v4/domains/"+url.PathEscape(id)+"/verify", nil)

	return parse("verify domain", resp, err)
}

/*
DeleteDomain removes the domain.
*/
func (d *Domains) DeleteDomain(ctx context.Context, id string) error {
	if id == "" {
		return provider.DomainNotFound("mailgun", "delete domain")
	}

	resp, err := d.request().Delete(ctx, d.endpoint+"/v3/domains/"+url.PathEscape(id))

	return provider.CheckDomain("mailgun", "delete domain", resp, err)
}

func (d *Domains) turnOffTracking(ctx context.Context, id string) error {
	for _, kind := range []string{"click", "open"} {
		target := d.endpoint + "/v3/domains/" + url.PathEscape(id) + "/tracking/" + kind
		resp, err := d.request().WithHeader("Content-Type", formEncoded).Put(ctx, target, "active=no")

		if err := provider.CheckDomain("mailgun", "turn off tracking", resp, err); err != nil {
			return err
		}
	}

	return nil
}

func (d *Domains) request() *httpclient.Request {
	credentials := base64.StdEncoding.EncodeToString([]byte("api:" + d.token))

	return d.client.New().WithHeader("Authorization", "Basic "+credentials).Accept("application/json")
}

func parse(action string, resp *httpclient.Response, err error) (mail.ProviderDomain, error) {
	if err := provider.CheckDomain("mailgun", action, resp, err); err != nil {
		return mail.ProviderDomain{}, err
	}

	var answer domainAnswer
	if err := provider.DecodeDomain("mailgun", action, resp, &answer); err != nil {
		return mail.ProviderDomain{}, err
	}

	return answer.domain(), nil
}

/*
domainAnswer is the domain Mailgun answers with.
*/
type domainAnswer struct {
	Domain struct {
		Name  string `json:"name"`
		State string `json:"state"`
	} `json:"domain"`
	Records []recordAnswer `json:"sending_dns_records"`
}

/*
recordAnswer is one sending record of a domain.
*/
type recordAnswer struct {
	Type  string `json:"record_type"`
	Name  string `json:"name"`
	Value string `json:"value"`
	Valid string `json:"valid"`
}

func (a domainAnswer) domain() mail.ProviderDomain {
	records := make([]mail.DomainRecord, 0, len(a.Records))
	for _, record := range a.Records {
		records = append(records, mail.DomainRecord{
			Purpose: record.purpose(),
			Type:    strings.ToUpper(record.Type),
			Host:    strings.TrimSuffix(record.Name, "."),
			Value:   record.Value,
			Valid:   record.Valid == recordValid,
		})
	}

	return mail.ProviderDomain{ID: a.Domain.Name, Verified: a.Domain.State == stateActive, Records: records}
}

/*
purpose reads the record's use from its content, as Mailgun does not name
it. A signing key may be a TXT or a CNAME, so its host is looked at
first.
*/
func (r recordAnswer) purpose() string {
	switch {
	case strings.Contains(r.Name, "._domainkey."):
		return mail.PurposeDKIM
	case strings.HasPrefix(r.Value, "v=spf1"):
		return mail.PurposeSPF
	case strings.EqualFold(r.Type, "CNAME"):
		return mail.PurposeTracking
	case strings.EqualFold(r.Type, "MX"):
		return mail.PurposeMX
	default:
		return mail.PurposeOther
	}
}
