/*
|--------------------------------------------------------------------------
| GIF Search Controller
|--------------------------------------------------------------------------
|
| What the composer needs to search GIFs. GIPHY's terms have the browser
| call GIPHY itself, never through us, so the key goes to the signed-in
| teammate's page. Without a key on the instance, or with the workspace's
| GIF picker off, the search is off and no key is given.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/support/gifs"
)

/*
GIFSearch is how the composer searches GIFs: whether it may, and then
the provider and its key.
*/
type GIFSearch struct {
	Object   string  `json:"object"`
	Enabled  bool    `json:"enabled"`
	Provider *string `json:"provider"`
	APIKey   *string `json:"api_key"`
}

/*
GIFSearchOutput is the GIF search set up for the workspace.
*/
type GIFSearchOutput struct {
	Body fhttp.Resource[GIFSearch]
}

/*
GIFSearchController answers with the GIF search of the workspace.
APIKey is the instance's GIPHY key, empty when GIF search is off.
*/
type GIFSearchController struct {
	APIKey string
}

/*
Show returns the GIF search, without a key when it is off.
*/
func (c *GIFSearchController) Show(ctx context.Context, _ *requests.NoInput) (*GIFSearchOutput, error) {
	search := GIFSearch{Object: "gif_search"}
	if c.APIKey != "" && membership(ctx).Workspace.GIFPickerOn() {
		search.Enabled, search.Provider, search.APIKey = true, new(gifs.Provider), new(c.APIKey)
	}

	return &GIFSearchOutput{Body: fhttp.NewResource(search)}, nil
}
