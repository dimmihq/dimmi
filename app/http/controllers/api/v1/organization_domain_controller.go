/*
|--------------------------------------------------------------------------
| Organization Domain Controller
|--------------------------------------------------------------------------
|
| GET /organization-domains lists the domains of the organization behind
| the workspace in play, POST adds one with the record that proves it,
| POST /organization-domains/{domain}/checks looks the proof up now,
| PATCH routes a verified domain to a connection and DELETE removes it.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/sso"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
DomainsOutput is every domain of the organization.
*/
type DomainsOutput struct {
	Body fhttp.Collection[resources.OrganizationDomain]
}

/*
DomainOutput is one domain.
*/
type DomainOutput struct {
	Body fhttp.Resource[resources.OrganizationDomain]
}

/*
DomainCheckOutput is what checking a domain found.
*/
type DomainCheckOutput struct {
	Body fhttp.Resource[resources.DomainCheck]
}

/*
OrganizationDomainController serves the domains of the organization
behind the workspace in play.
*/
type OrganizationDomainController struct {
	Domains *sso.Domains
	IDs     *ids.Registry
}

/*
Index lists the domains.
*/
func (c *OrganizationDomainController) Index(ctx context.Context, _ *requests.NoInput) (*DomainsOutput, error) {
	rows, err := c.Domains.List(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}

	items := make([]resources.OrganizationDomain, 0, len(rows))
	for _, row := range rows {
		items = append(items, resources.NewOrganizationDomain(row))
	}

	page := pagination.Page[resources.OrganizationDomain]{Items: items, Limit: len(items)}

	return &DomainsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Store adds a domain and answers with the record that proves it.
*/
func (c *OrganizationDomainController) Store(
	ctx context.Context, in *requests.StoreDomainRequest,
) (*DomainOutput, error) {
	domain, err := c.Domains.Add(ctx, membership(ctx), in.Body.Domain)
	if err != nil {
		return nil, err
	}

	return &DomainOutput{Body: fhttp.NewResource(resources.NewOrganizationDomain(domain))}, nil
}

/*
Check looks the domain's proof up and answers with what it found.
*/
func (c *OrganizationDomainController) Check(
	ctx context.Context, in *requests.DomainRequest,
) (*DomainCheckOutput, error) {
	id, err := decodeID(c.IDs, models.PrefixOrganizationDomain, in.Domain)
	if err != nil {
		return nil, err
	}

	check, err := c.Domains.Verify(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}

	shaped := resources.NewOrganizationDomain(check.Domain)

	return &DomainCheckOutput{Body: fhttp.NewResource(resources.DomainCheck{
		Object: "organization_domain_check", Status: shaped.Status, FoundRecords: check.Found, Domain: shaped,
	})}, nil
}

/*
Update routes the domain to a connection, or to none.
*/
func (c *OrganizationDomainController) Update(
	ctx context.Context, in *requests.UpdateDomainRequest,
) (*DomainOutput, error) {
	id, err := decodeID(c.IDs, models.PrefixOrganizationDomain, in.Domain)
	if err != nil {
		return nil, err
	}

	var connection *ids.UUID

	if in.Body.SSOConnectionID != nil {
		decoded, err := decodeID(c.IDs, models.PrefixSSOConnection, *in.Body.SSOConnectionID)
		if err != nil {
			return nil, err
		}

		connection = &decoded
	}

	domain, err := c.Domains.Route(ctx, membership(ctx), id, connection)
	if err != nil {
		return nil, err
	}

	return &DomainOutput{Body: fhttp.NewResource(resources.NewOrganizationDomain(domain))}, nil
}

/*
Destroy removes the domain.
*/
func (c *OrganizationDomainController) Destroy(
	ctx context.Context, in *requests.DomainRequest,
) (*EmptyOutput, error) {
	id, err := decodeID(c.IDs, models.PrefixOrganizationDomain, in.Domain)
	if err != nil {
		return nil, err
	}

	if err := c.Domains.Remove(ctx, membership(ctx), id); err != nil {
		return nil, err
	}

	return &EmptyOutput{}, nil
}
