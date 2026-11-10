/*
|--------------------------------------------------------------------------
| Broadcast Links
|--------------------------------------------------------------------------
|
| The links inside broadcast emails, answered without signing in. The
| open pixel always answers with an image and the click link always
| redirects, even when the link is unknown, so a broken link never
| shows a recipient an error page. The one-click unsubscribe takes a
| mail app's POST as RFC 8058 describes; the preference page link opens
| the page in the app.
|
*/

package web

import (
	"encoding/base64"
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/broadcasts"
)

/*
pixel is a transparent one-pixel GIF.
*/
const pixel = "R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7"

/*
BroadcastLinkController answers the public links in broadcast emails.
*/
type BroadcastLinkController struct {
	Broadcasts *broadcasts.Broadcasts
	AppURL     string
}

/*
Open records an open and answers with the pixel.
*/
func (c BroadcastLinkController) Open(w fhttp.ResponseWriter, r *fhttp.Request) {
	token := strings.TrimSuffix(r.PathValue("token"), ".gif")
	_ = c.Broadcasts.Opened(r.Context(), token)

	gif, _ := base64.StdEncoding.DecodeString(pixel)
	w.Header().Set("Content-Type", "image/gif")
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(fhttp.StatusOK)
	_, _ = w.Write(gif)
}

/*
Click records a click and redirects to the link.
*/
func (c BroadcastLinkController) Click(w fhttp.ResponseWriter, r *fhttp.Request) {
	url, err := c.Broadcasts.Clicked(r.Context(), r.PathValue("token"))
	if err != nil || url == "" {
		url = c.AppURL
	}

	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Location", url)
	w.WriteHeader(fhttp.StatusFound)
}

/*
Preferences opens the preference page for the link.
*/
func (c BroadcastLinkController) Preferences(w fhttp.ResponseWriter, r *fhttp.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Location", "/email-preferences?t="+r.PathValue("token"))
	w.WriteHeader(fhttp.StatusFound)
}

/*
OneClick unsubscribes as a mail app's one-click button asks.
*/
func (c BroadcastLinkController) OneClick(w fhttp.ResponseWriter, r *fhttp.Request) {
	if err := c.Broadcasts.OneClick(r.Context(), r.PathValue("token")); err != nil {
		w.WriteHeader(fhttp.StatusNotFound)

		return
	}

	w.WriteHeader(fhttp.StatusOK)
}
