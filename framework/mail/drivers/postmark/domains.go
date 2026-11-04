/*
|--------------------------------------------------------------------------
| Postmark Domains
|--------------------------------------------------------------------------
|
| Postmark manages domains with an account token, which reaches further
| than the server token that sends mail. A new domain's signing key waits
| as a pending key until it is seen in DNS, and during a key rotation the
| active and the pending key are both needed, so both are reported.
| Signing and the return path are verified by separate calls.
|
| Postmark answers 422 with its own error code for a domain it does not
| have. Tracking is set per message, so a domain has none to turn off.
|
*/

package postmark

import (
	"cmp"
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/mail/internal/provider"
)

/*
codeDomainNotFound is Postmark's error code for a domain it does not
have.
*/
const codeDomainNotFound = 510

/*
Domains manages sending domains through the Postmark API.
*/
type Domains struct {
	client   *httpclient.Client
	token    string
	endpoint string
}

/*
NewDomains returns a domain manager using the account token in the
options.
*/
func NewDomains(client *httpclient.Client, opts mail.DomainOptions) *Domains {
	return &Domains{client: client, token: opts.Token, endpoint: cmp.Or(opts.Endpoint, "https://api.postmarkapp.com")}
}

/*
CreateDomain posts the domain, leaving the default return path.
*/
func (d *Domains) CreateDomain(ctx context.Context, domain string) (mail.ProviderDomain, error) {
	resp, err := d.request().Post(ctx, d.endpoint+"/domains", map[string]string{"Name": domain})

	return parse("create domain", resp, err)
}

/*
GetDomain reads the domain.
*/
func (d *Domains) GetDomain(ctx context.Context, id string) (mail.ProviderDomain, error) {
	if id == "" {
		return mail.ProviderDomain{}, provider.DomainNotFound("postmark", "get domain")
	}

	resp, err := d.request().Get(ctx, d.url(id))

	return parse("get domain", resp, err)
}

/*
VerifyDomain checks the signing key and then the return path. The second
answer carries the outcome of both.
*/
func (d *Domains) VerifyDomain(ctx context.Context, id string) (mail.ProviderDomain, error) {
	if id == "" {
		return mail.ProviderDomain{}, provider.DomainNotFound("postmark", "verify domain")
	}

	resp, err := d.request().Put(ctx, d.url(id)+"/verifyDkim", nil)
	if err := check("verify domain", resp, err); err != nil {
		return mail.ProviderDomain{}, err
	}

	resp, err = d.request().Put(ctx, d.url(id)+"/verifyReturnPath", nil)

	return parse("verify domain", resp, err)
}

/*
DeleteDomain removes the domain.
*/
func (d *Domains) DeleteDomain(ctx context.Context, id string) error {
	if id == "" {
		return provider.DomainNotFound("postmark", "delete domain")
	}

	resp, err := d.request().Delete(ctx, d.url(id))

	return check("delete domain", resp, err)
}

func (d *Domains) request() *httpclient.Request {
	return d.client.New().WithHeader("X-Postmark-Account-Token", d.token).Accept("application/json")
}

func (d *Domains) url(id string) string {
	return d.endpoint + "/domains/" + url.PathEscape(id)
}

func check(action string, resp *httpclient.Response, err error) error {
	if err == nil && resp.Status == http.StatusUnprocessableEntity {
		var refusal struct{ ErrorCode int }
		if resp.JSON(&refusal) == nil && refusal.ErrorCode == codeDomainNotFound {
			return provider.DomainNotFound("postmark", action)
		}
	}

	return provider.CheckDomain("postmark", action, resp, err)
}

func parse(action string, resp *httpclient.Response, err error) (mail.ProviderDomain, error) {
	if err := check(action, resp, err); err != nil {
		return mail.ProviderDomain{}, err
	}

	var answer domainAnswer
	if err := provider.DecodeDomain("postmark", action, resp, &answer); err != nil {
		return mail.ProviderDomain{}, err
	}

	return answer.domain(), nil
}

/*
domainAnswer is the domain Postmark answers with. Postmark's field names
decode without tags.
*/
type domainAnswer struct {
	ID                         int64
	DKIMVerified               bool
	DKIMHost                   string
	DKIMTextValue              string
	DKIMPendingHost            string
	DKIMPendingTextValue       string
	ReturnPathDomain           string
	ReturnPathDomainVerified   bool
	ReturnPathDomainCNAMEValue string
}

func (a domainAnswer) domain() mail.ProviderDomain {
	return mail.ProviderDomain{
		ID:       strconv.FormatInt(a.ID, 10),
		Verified: a.DKIMVerified && a.ReturnPathDomainVerified,
		Records:  a.records(),
	}
}

func (a domainAnswer) records() []mail.DomainRecord {
	var records []mail.DomainRecord
	if a.DKIMHost != "" {
		records = append(records, mail.DomainRecord{
			Purpose: mail.PurposeDKIM, Type: "TXT", Host: a.DKIMHost, Value: a.DKIMTextValue, Valid: a.DKIMVerified,
		})
	}

	if a.DKIMPendingHost != "" {
		records = append(records, mail.DomainRecord{
			Purpose: mail.PurposeDKIM, Type: "TXT", Host: a.DKIMPendingHost, Value: a.DKIMPendingTextValue,
		})
	}

	if a.ReturnPathDomain != "" {
		records = append(records, mail.DomainRecord{
			Purpose: mail.PurposeReturnPath, Type: "CNAME", Host: a.ReturnPathDomain,
			Value: a.ReturnPathDomainCNAMEValue, Valid: a.ReturnPathDomainVerified,
		})
	}

	return records
}
