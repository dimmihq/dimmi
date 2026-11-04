/*
|--------------------------------------------------------------------------
| Resolve Branding
|--------------------------------------------------------------------------
|
| Whether a messenger shows the "Powered by" badge. On a self-hosted install
| it is hidden unless someone opts in. On the cloud it stays, locked, until
| the workspace has paid any invoice, and after that it is the customer's
| choice. Nothing else may decide this.
|
*/

package widget

import (
	"context"
	"net/url"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/config"
)

/*
Branding is what the badge does: whether it shows, whether the customer may
hide it, and where it links.
*/
type Branding struct {
	Show   bool   `json:"show"`
	Locked bool   `json:"locked"`
	URL    string `json:"url"`
}

/*
ResolveBranding works out a widget's badge.
*/
type ResolveBranding struct {
	Edition config.Edition
	Billing contracts.BillingStatus
}

/*
Handle returns the badge of a widget of the workspace given what the widget
chose; nil follows the edition.
*/
func (a *ResolveBranding) Handle(ctx context.Context, w models.Workspace, show *bool) (Branding, error) {
	out := Branding{URL: badgeURL(w.Slug)}
	chose := show != nil && *show

	if a.Edition != config.EditionCloud {
		out.Show = chose

		return out, nil
	}

	paid, err := a.Billing.HasPaid(ctx, w.ID)
	if err != nil {
		return out, err
	}

	out.Show, out.Locked = chose || !paid, !paid

	return out, nil
}

/*
badgeURL returns where the badge of the workspace links.
*/
func badgeURL(slug string) string {
	query := url.Values{"utm_source": {"widget"}, "utm_medium": {"badge"}, "ref": {slug}}

	return "https://trydimmi.com/?" + query.Encode()
}

/*
NeverPaid is the billing status of an install without billing.
*/
type NeverPaid struct{}

/*
HasPaid answers no.
*/
func (NeverPaid) HasPaid(context.Context, ids.UUID) (bool, error) {
	return false, nil
}
