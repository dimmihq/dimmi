/*
|--------------------------------------------------------------------------
| Resend Domains
|--------------------------------------------------------------------------
|
| Resend answers with record names relative to the domain, which are made
| fully qualified here so every driver reports hosts alike. Asking it to
| verify only starts a check in the background and returns no state, so
| the domain is read again afterwards. Tracking is switched off in the
| same request that creates the domain.
|
*/

package resend

import (
	"cmp"
	"context"
	"net/url"
	"strings"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/mail/internal/provider"
)

/*
statusVerified is the status Resend gives a domain or record it found in
DNS.
*/
const statusVerified = "verified"

/*
Domains manages sending domains through the Resend API.
*/
type Domains struct {
	client   *httpclient.Client
	token    string
	endpoint string
}

/*
NewDomains returns a domain manager using the API key in the options.
*/
func NewDomains(client *httpclient.Client, opts mail.DomainOptions) *Domains {
	return &Domains{client: client, token: opts.Token, endpoint: cmp.Or(opts.Endpoint, "https://api.resend.com")}
}

/*
CreateDomain posts the domain with tracking off.
*/
func (d *Domains) CreateDomain(ctx context.Context, domain string) (mail.ProviderDomain, error) {
	resp, err := d.request().Post(ctx, d.endpoint+"/domains", map[string]any{
		"name": domain, "open_tracking": false, "click_tracking": false,
	})

	return parse("create domain", resp, err)
}

/*
GetDomain reads the domain.
*/
func (d *Domains) GetDomain(ctx context.Context, id string) (mail.ProviderDomain, error) {
	if id == "" {
		return mail.ProviderDomain{}, provider.DomainNotFound("resend", "get domain")
	}

	resp, err := d.request().Get(ctx, d.url(id))

	return parse("get domain", resp, err)
}

/*
VerifyDomain starts a check and reads the domain back.
*/
func (d *Domains) VerifyDomain(ctx context.Context, id string) (mail.ProviderDomain, error) {
	if id == "" {
		return mail.ProviderDomain{}, provider.DomainNotFound("resend", "verify domain")
	}

	resp, err := d.request().Post(ctx, d.url(id)+"/verify", nil)
	if err := provider.CheckDomain("resend", "verify domain", resp, err); err != nil {
		return mail.ProviderDomain{}, err
	}

	return d.GetDomain(ctx, id)
}

/*
DeleteDomain removes the domain.
*/
func (d *Domains) DeleteDomain(ctx context.Context, id string) error {
	if id == "" {
		return provider.DomainNotFound("resend", "delete domain")
	}

	resp, err := d.request().Delete(ctx, d.url(id))

	return provider.CheckDomain("resend", "delete domain", resp, err)
}

func (d *Domains) request() *httpclient.Request {
	return d.client.New().WithToken(d.token).Accept("application/json")
}

func (d *Domains) url(id string) string {
	return d.endpoint + "/domains/" + url.PathEscape(id)
}

func parse(action string, resp *httpclient.Response, err error) (mail.ProviderDomain, error) {
	if err := provider.CheckDomain("resend", action, resp, err); err != nil {
		return mail.ProviderDomain{}, err
	}

	var answer domainAnswer
	if err := provider.DecodeDomain("resend", action, resp, &answer); err != nil {
		return mail.ProviderDomain{}, err
	}

	return answer.domain(), nil
}

/*
domainAnswer is the domain Resend answers with.
*/
type domainAnswer struct {
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	Status  string         `json:"status"`
	Records []recordAnswer `json:"records"`
}

/*
recordAnswer is one record of a domain. Kind is what Resend calls the
record's use, such as DKIM or SPF.
*/
type recordAnswer struct {
	Kind     string `json:"record"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Value    string `json:"value"`
	Status   string `json:"status"`
	Priority int    `json:"priority"`
}

func (a domainAnswer) domain() mail.ProviderDomain {
	records := make([]mail.DomainRecord, 0, len(a.Records))
	for _, record := range a.Records {
		records = append(records, mail.DomainRecord{
			Purpose:  record.purpose(),
			Type:     strings.ToUpper(record.Type),
			Host:     provider.FullHost(record.Name, a.Name),
			Value:    strings.Trim(record.Value, `"`),
			Priority: record.Priority,
			Valid:    record.Status == statusVerified,
		})
	}

	return mail.ProviderDomain{ID: a.ID, Verified: a.Status == statusVerified, Records: records}
}

/*
purpose reads the record's use. Resend files the bounce MX record under
SPF, so the type decides first.
*/
func (r recordAnswer) purpose() string {
	switch {
	case strings.EqualFold(r.Type, "MX"):
		return mail.PurposeMX
	case strings.EqualFold(r.Kind, "DKIM"):
		return mail.PurposeDKIM
	case strings.EqualFold(r.Kind, "SPF"):
		return mail.PurposeSPF
	case strings.EqualFold(r.Kind, "Tracking"):
		return mail.PurposeTracking
	default:
		return mail.PurposeOther
	}
}
