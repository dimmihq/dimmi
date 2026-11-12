/*
|--------------------------------------------------------------------------
| Help Center Site Controller
|--------------------------------------------------------------------------
|
| Adding and deleting a workspace's help centers, the custom domain a
| help center answers on, and what readers did there.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
HelpCenterSiteController serves help centers' lifecycle, domains and
analytics.
*/
type HelpCenterSiteController struct {
	HelpCenters *helpcenters.HelpCenters
	Domains     *helpcenters.Domains
	Counts      *helpcenters.Analytics
	Addresses   helpcenters.Addresses
	IDs         *ids.Registry
}

/*
HelpCenterDomainOutput is a help center's custom domain, null without
one.
*/
type HelpCenterDomainOutput struct {
	Body struct {
		Data *resources.HelpCenterDomain `json:"data"`
	}
}

/*
HelpCenterAnalyticsOutput is a help center's analytics.
*/
type HelpCenterAnalyticsOutput struct {
	Body fhttp.Resource[resources.HelpCenterAnalytics]
}

/*
Store adds a help center.
*/
func (c *HelpCenterSiteController) Store(ctx context.Context, in *requests.StoreHelpCenterRequest,
) (*HelpCenterOutput, error) {
	h, err := c.HelpCenters.Create(ctx, helpcenters.NewHelpCenter{
		Membership: membership(ctx), Name: in.Body.Name, Subdomain: in.Body.Subdomain, Locale: in.Body.Locale,
	})
	if err != nil {
		return nil, err
	}
	return &HelpCenterOutput{Body: fhttp.NewResource(resources.NewHelpCenter(h, c.Addresses.Root(h)))}, nil
}

/*
Destroy deletes a help center that is not the default.
*/
func (c *HelpCenterSiteController) Destroy(ctx context.Context, in *requests.DestroyHelpCenterRequest,
) (*struct{}, error) {
	id, err := c.id(in.HelpCenter)
	if err != nil {
		return nil, err
	}
	return &struct{}{}, c.HelpCenters.Delete(ctx, membership(ctx), id, in.Confirm)
}

/*
ShowDomain answers the help center's custom domain.
*/
func (c *HelpCenterSiteController) ShowDomain(ctx context.Context, in *requests.HelpCenterRequest,
) (*HelpCenterDomainOutput, error) {
	id, err := c.id(in.HelpCenter)
	if err != nil {
		return nil, err
	}
	d, found, err := c.Domains.Show(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}
	out := &HelpCenterDomainOutput{}
	if found {
		out.Body.Data = new(resources.NewHelpCenterDomain(d))
	}
	return out, nil
}

/*
StoreDomain claims a custom domain for the help center.
*/
func (c *HelpCenterSiteController) StoreDomain(ctx context.Context, in *requests.StoreHelpCenterDomainRequest,
) (*HelpCenterDomainOutput, error) {
	id, err := c.id(in.HelpCenter)
	if err != nil {
		return nil, err
	}
	d, err := c.Domains.Attach(ctx, membership(ctx), id, in.Body.Hostname)
	return domainOutput(d, err)
}

/*
CheckDomain checks the help center's custom domain again now.
*/
func (c *HelpCenterSiteController) CheckDomain(ctx context.Context, in *requests.HelpCenterRequest,
) (*HelpCenterDomainOutput, error) {
	id, err := c.id(in.HelpCenter)
	if err != nil {
		return nil, err
	}
	d, err := c.Domains.Recheck(ctx, membership(ctx), id)
	return domainOutput(d, err)
}

/*
DestroyDomain removes the help center's custom domain.
*/
func (c *HelpCenterSiteController) DestroyDomain(ctx context.Context, in *requests.HelpCenterRequest,
) (*struct{}, error) {
	id, err := c.id(in.HelpCenter)
	if err != nil {
		return nil, err
	}
	return &struct{}{}, c.Domains.Remove(ctx, membership(ctx), id)
}

/*
Analytics answers what readers did over the last days.
*/
func (c *HelpCenterSiteController) Analytics(ctx context.Context, in *requests.HelpCenterAnalyticsRequest,
) (*HelpCenterAnalyticsOutput, error) {
	id, err := c.id(in.HelpCenter)
	if err != nil {
		return nil, err
	}
	days := in.Days
	if days == 0 {
		days = 30
	}
	report, err := c.Counts.Report(ctx, membership(ctx), id, days)
	if err != nil {
		return nil, err
	}
	return &HelpCenterAnalyticsOutput{Body: fhttp.NewResource(resources.NewHelpCenterAnalytics(report))}, nil
}

func domainOutput(d helpcenters.Domain, err error) (*HelpCenterDomainOutput, error) {
	if err != nil {
		return nil, err
	}
	out := &HelpCenterDomainOutput{}
	out.Body.Data = new(resources.NewHelpCenterDomain(d))
	return out, nil
}

func (c *HelpCenterSiteController) id(public string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(models.PrefixHelpCenter), public)
	if err != nil {
		return ids.UUID{}, exceptions.ErrNotFound
	}
	return id, nil
}
