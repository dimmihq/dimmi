/*
|--------------------------------------------------------------------------
| Widget Help Pass Controller
|--------------------------------------------------------------------------
|
| Where the messenger opens an article in the full help center. A visitor
| the messenger knows gets an address with a pass in its fragment, which
| works once and for a minute and signs them in on arrival; anyone else
| gets the article's plain address.
|
*/

package widget

import (
	"context"
	"net/url"

	"github.com/dimmihq/dimmi/framework/fault"
	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/http/requests"
)

/*
ErrNoHelpCenter says the messenger links to no help center.
*/
var ErrNoHelpCenter = fault.New(fault.NotFound, "help_center_not_found",
	"This messenger links to no help center.")

/*
HelpPass is where to open the help center.
*/
type HelpPass struct {
	Object string `json:"object"`
	URL    string `json:"url"`
}

/*
HelpPassOutput answers a pass asked for.
*/
type HelpPassOutput struct {
	Status int
	Body   fhttp.Resource[HelpPass]
}

/*
Pass returns where to open the article, signed in when the messenger
knows the visitor.
*/
func (c *HelpController) Pass(ctx context.Context, in *requests.StoreWidgetHelpPassRequest) (*HelpPassOutput, error) {
	auth := authenticated(ctx)
	if auth.Resolved.Widget.HelpCenterID == nil || c.Public == nil {
		return nil, ErrNoHelpCenter
	}

	snap, err := c.Public.LoadByID(ctx, auth.Resolved.Workspace.ID, *auth.Resolved.Widget.HelpCenterID)
	if err != nil {
		return nil, ErrNoHelpCenter
	}

	root := c.Addresses.Root(snap.HelpCenter)
	page := root
	if in.Body.Article != "" {
		page += "/articles/" + in.Body.Article
	}

	out := &HelpPassOutput{
		Status: fhttp.StatusCreated, Body: fhttp.NewResource(HelpPass{Object: "help_pass", URL: page}),
	}
	reader, known := c.known(auth)
	if !known {
		return out, nil
	}

	reader.Center = snap.HelpCenter
	pass, err := c.Readers.IssuePass(ctx, reader)
	if err != nil {
		return nil, err
	}

	out.Body.Data.URL = root + "/sso?next=" + url.QueryEscape(pathOf(page)) + "#pass=" + pass

	return out, nil
}

/*
pathOf is the path part of an address, the home path for none.
*/
func pathOf(address string) string {
	u, err := url.Parse(address)
	if err != nil || u.Path == "" {
		return "/"
	}

	return u.Path
}
