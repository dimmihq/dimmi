/*
|--------------------------------------------------------------------------
| SendGrid Domains
|--------------------------------------------------------------------------
|
| SendGrid manages the records of an authenticated domain itself, leaving
| three CNAMEs to publish. Asking it to validate answers with results but
| not the records, so the domain is read again afterwards.
|
| SendGrid has no tracking per domain, only for the whole account, so
| creating a domain turns off click and open tracking for the account.
| That needs a wider scope on the key than domain authentication alone;
| without it the domain comes back with ErrDomainTrackingLeftOn. Messages
| from this driver turn tracking off for themselves either way.
|
*/

package sendgrid

import (
	"cmp"
	"context"
	"net/url"
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/mail/internal/provider"
)

/*
Domains manages sending domains through the SendGrid API.
*/
type Domains struct {
	client   *httpclient.Client
	token    string
	endpoint string
}

/*
NewDomains returns a domain manager using the API key in the options.
Set the endpoint to https://api.eu.sendgrid.com for the EU region.
*/
func NewDomains(client *httpclient.Client, opts mail.DomainOptions) *Domains {
	return &Domains{client: client, token: opts.Token, endpoint: cmp.Or(opts.Endpoint, "https://api.sendgrid.com")}
}

/*
CreateDomain posts the domain and then turns tracking off. When that
fails, the domain comes back with ErrDomainTrackingLeftOn.
*/
func (d *Domains) CreateDomain(ctx context.Context, domain string) (mail.ProviderDomain, error) {
	resp, err := d.request().Post(ctx, d.endpoint+"/v3/whitelabel/domains", map[string]any{
		"domain": domain, "automatic_security": true,
	})

	created, err := parse("create domain", resp, err)
	if err != nil {
		return mail.ProviderDomain{}, err
	}

	if err := d.turnOffTracking(ctx); err != nil {
		return created, provider.TrackingLeftOn("sendgrid", err)
	}

	return created, nil
}

/*
GetDomain reads the domain.
*/
func (d *Domains) GetDomain(ctx context.Context, id string) (mail.ProviderDomain, error) {
	if id == "" {
		return mail.ProviderDomain{}, provider.DomainNotFound("sendgrid", "get domain")
	}

	resp, err := d.request().Get(ctx, d.url(id))

	return parse("get domain", resp, err)
}

/*
VerifyDomain asks SendGrid to validate and reads the domain back.
*/
func (d *Domains) VerifyDomain(ctx context.Context, id string) (mail.ProviderDomain, error) {
	if id == "" {
		return mail.ProviderDomain{}, provider.DomainNotFound("sendgrid", "verify domain")
	}

	resp, err := d.request().Post(ctx, d.url(id)+"/validate", nil)
	if err := provider.CheckDomain("sendgrid", "verify domain", resp, err); err != nil {
		return mail.ProviderDomain{}, err
	}

	return d.GetDomain(ctx, id)
}

/*
DeleteDomain removes the domain.
*/
func (d *Domains) DeleteDomain(ctx context.Context, id string) error {
	if id == "" {
		return provider.DomainNotFound("sendgrid", "delete domain")
	}

	resp, err := d.request().Delete(ctx, d.url(id))

	return provider.CheckDomain("sendgrid", "delete domain", resp, err)
}

func (d *Domains) turnOffTracking(ctx context.Context) error {
	for _, kind := range []string{"click", "open"} {
		target := d.endpoint + "/v3/tracking_settings/" + kind
		resp, err := d.request().Patch(ctx, target, map[string]bool{"enabled": false})

		if err := provider.CheckDomain("sendgrid", "turn off tracking", resp, err); err != nil {
			return err
		}
	}

	return nil
}

func (d *Domains) request() *httpclient.Request {
	return d.client.New().WithToken(d.token).Accept("application/json")
}

func (d *Domains) url(id string) string {
	return d.endpoint + "/v3/whitelabel/domains/" + url.PathEscape(id)
}

func parse(action string, resp *httpclient.Response, err error) (mail.ProviderDomain, error) {
	if err := provider.CheckDomain("sendgrid", action, resp, err); err != nil {
		return mail.ProviderDomain{}, err
	}

	var answer domainAnswer
	if err := provider.DecodeDomain("sendgrid", action, resp, &answer); err != nil {
		return mail.ProviderDomain{}, err
	}

	return answer.domain(), nil
}

/*
domainAnswer is the authenticated domain SendGrid answers with.
*/
type domainAnswer struct {
	ID    int64 `json:"id"`
	Valid bool  `json:"valid"`
	DNS   struct {
		MailCNAME recordAnswer `json:"mail_cname"`
		DKIM1     recordAnswer `json:"dkim1"`
		DKIM2     recordAnswer `json:"dkim2"`
	} `json:"dns"`
}

/*
recordAnswer is one record of a domain.
*/
type recordAnswer struct {
	Valid bool   `json:"valid"`
	Type  string `json:"type"`
	Host  string `json:"host"`
	Data  string `json:"data"`
}

func (a domainAnswer) domain() mail.ProviderDomain {
	return mail.ProviderDomain{
		ID:       strconv.FormatInt(a.ID, 10),
		Verified: a.Valid,
		Records: []mail.DomainRecord{
			a.DNS.MailCNAME.record(mail.PurposeReturnPath),
			a.DNS.DKIM1.record(mail.PurposeDKIM),
			a.DNS.DKIM2.record(mail.PurposeDKIM),
		},
	}
}

func (r recordAnswer) record(purpose string) mail.DomainRecord {
	return mail.DomainRecord{
		Purpose: purpose,
		Type:    strings.ToUpper(r.Type),
		Host:    r.Host,
		Value:   r.Data,
		Valid:   r.Valid,
	}
}
