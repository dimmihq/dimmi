/*
|--------------------------------------------------------------------------
| SSO Lookup Controller
|--------------------------------------------------------------------------
|
| POST /sso-lookups tells the sign-in page whether an address's company
| signs in through its identity provider, and where to send the person.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/ssologin"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/http/requests"
)

/*
SSOLookup is where an address signs in: the connection's name and
address, null for none, and whether the company requires it.
*/
type SSOLookup struct {
	Object      string  `json:"object"`
	RedirectURL *string `json:"redirect_url"`
	Name        *string `json:"name"`
	Enforced    bool    `json:"enforced"`
}

/*
SSOLookupOutput answers a lookup.
*/
type SSOLookupOutput struct {
	Body fhttp.Resource[SSOLookup]
}

/*
SSOLookupController answers lookups.
*/
type SSOLookupController struct {
	Lookup  *ssologin.Lookup
	Cookies appauth.Cookies
}

/*
Store looks the address up.
*/
func (c *SSOLookupController) Store(ctx context.Context, in *requests.StoreSSOLookupRequest) (*SSOLookupOutput, error) {
	route, found, err := c.Lookup.Handle(ctx, ssologin.LookupInput{
		Email: in.Body.Email, Client: clientOf(ctx, c.Cookies),
	})
	if err != nil {
		return nil, err
	}

	out := SSOLookup{Object: "sso_lookup"}
	if found {
		out.RedirectURL, out.Name, out.Enforced = &route.RedirectURL, &route.Name, route.Enforced
	}

	return &SSOLookupOutput{Body: fhttp.NewResource(out)}, nil
}
