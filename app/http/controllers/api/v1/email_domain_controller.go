/*
|--------------------------------------------------------------------------
| Email Domain Controller
|--------------------------------------------------------------------------
|
| The sending domains of the workspace in play: listing them, adding one
| with the records to publish, asking for a check, changing the return
| path or DNS host, rotating a relay's key, and removing one. A single
| domain also comes as a zone file, for hosts that import one. The DNS
| host is read from the domain's name servers whenever one domain is
| shown. Ids of other workspaces' domains are simply not found.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/email"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/integrations/email/dns"
	"github.com/dimmihq/dimmi/app/models"
)

/*
EmailDomainsOutput is the workspace's domains.
*/
type EmailDomainsOutput struct {
	Body fhttp.Collection[resources.EmailDomain]
}

/*
EmailDomainOutput is one domain.
*/
type EmailDomainOutput struct {
	Body fhttp.Resource[resources.EmailDomain]
}

/*
EmailDomainShowOutput is one domain, or its records as a zone file.
*/
type EmailDomainShowOutput struct {
	ContentType string `header:"Content-Type"`
	Body        any
}

/*
EmailDomainController serves the workspace's sending domains. Hosts reads
who hosts a domain's DNS, and Base is the server's own address, which
the domain's links start with.
*/
type EmailDomainController struct {
	Create *email.CreateEmailDomain
	List   *email.ListEmailDomains
	Show   *email.ShowEmailDomain
	Check  *email.RequestEmailDomainCheck
	Change *email.UpdateEmailDomain
	Remove *email.DeleteEmailDomain
	Rotate *email.RotateEmailDomainKey
	Hosts  func(ctx context.Context, domain string) string
	IDs    *ids.Registry
	Base   string
}

/*
Index lists the domains by name.
*/
func (c *EmailDomainController) Index(ctx context.Context, _ *requests.NoInput) (*EmailDomainsOutput, error) {
	rows, err := c.List.Handle(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}

	items := resources.NewEmailDomains(ctx, rows, c.Base)

	return &EmailDomainsOutput{Body: fhttp.NewCollection(pagination.Page[resources.EmailDomain]{
		Items: items, Limit: len(items),
	})}, nil
}

/*
Store adds a domain and answers with the records to publish.
*/
func (c *EmailDomainController) Store(
	ctx context.Context, in *requests.StoreEmailDomainRequest,
) (*EmailDomainOutput, error) {
	input := email.CreateEmailDomainInput{Membership: membership(ctx), Domain: in.Body.Domain}
	if in.Body.MailFromSubdomain != nil {
		input.MailFrom = *in.Body.MailFromSubdomain
	}

	row, err := c.Create.Handle(ctx, input)
	if err != nil {
		return nil, err
	}

	return c.answer(ctx, row), nil
}

/*
ShowDomain answers with one domain, or its records as a zone file.
*/
func (c *EmailDomainController) ShowDomain(
	ctx context.Context, in *requests.ShowEmailDomainRequest,
) (*EmailDomainShowOutput, error) {
	id, err := c.domainID(in.Domain)
	if err != nil {
		return nil, err
	}

	row, err := c.Show.Handle(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}

	if in.Format == "bind" {
		return &EmailDomainShowOutput{
			ContentType: "text/plain; charset=utf-8", Body: []byte(dns.BIND(email.Records(row))),
		}, nil
	}

	return &EmailDomainShowOutput{ContentType: "application/json", Body: c.answer(ctx, row).Body}, nil
}

/*
StoreCheck queues a check of the domain now.
*/
func (c *EmailDomainController) StoreCheck(
	ctx context.Context, in *requests.EmailDomainRequest,
) (*EmailDomainOutput, error) {
	return c.act(ctx, in.Domain, func(id ids.UUID) (models.EmailDomain, error) {
		return c.Check.Handle(ctx, membership(ctx), id)
	})
}

/*
Update changes the return path or the DNS host.
*/
func (c *EmailDomainController) Update(
	ctx context.Context, in *requests.UpdateEmailDomainRequest,
) (*EmailDomainOutput, error) {
	return c.act(ctx, in.Domain, func(id ids.UUID) (models.EmailDomain, error) {
		return c.Change.Handle(ctx, email.UpdateEmailDomainInput{
			Membership: membership(ctx), ID: id, MailFrom: in.Body.MailFromSubdomain, DNSHost: in.Body.DNSHost,
		})
	})
}

/*
StoreRotation starts a new signing key.
*/
func (c *EmailDomainController) StoreRotation(
	ctx context.Context, in *requests.EmailDomainRequest,
) (*EmailDomainOutput, error) {
	return c.act(ctx, in.Domain, func(id ids.UUID) (models.EmailDomain, error) {
		return c.Rotate.Handle(ctx, membership(ctx), id)
	})
}

/*
Destroy removes the domain.
*/
func (c *EmailDomainController) Destroy(
	ctx context.Context, in *requests.DestroyEmailDomainRequest,
) (*struct{}, error) {
	id, err := c.domainID(in.Domain)
	if err != nil {
		return nil, err
	}

	err = c.Remove.Handle(ctx, email.DeleteEmailDomainInput{Membership: membership(ctx), ID: id, Force: in.Force})
	if err != nil {
		return nil, err
	}

	return &struct{}{}, nil
}

/*
act runs an action on the domain named in the path and answers with it.
*/
func (c *EmailDomainController) act(
	ctx context.Context, public string, run func(id ids.UUID) (models.EmailDomain, error),
) (*EmailDomainOutput, error) {
	id, err := c.domainID(public)
	if err != nil {
		return nil, err
	}

	row, err := run(id)
	if err != nil {
		return nil, err
	}

	return c.answer(ctx, row), nil
}

/*
answer shows the domain with the guide of its DNS host.
*/
func (c *EmailDomainController) answer(ctx context.Context, row models.EmailDomain) *EmailDomainOutput {
	hint := ""
	if c.Hosts != nil {
		hint = c.Hosts(ctx, row.Domain)
	}

	return &EmailDomainOutput{Body: fhttp.NewResource(resources.NewEmailDomain(ctx, row, hint, c.Base))}
}

/*
domainID decodes a domain's public id; a malformed one is not found.
*/
func (c *EmailDomainController) domainID(public string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(models.PrefixEmailDomain), public)
	if err != nil {
		return ids.UUID{}, exceptions.ErrNotFound
	}

	return id, nil
}
