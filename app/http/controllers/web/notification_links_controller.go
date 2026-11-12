/*
|--------------------------------------------------------------------------
| Notification Links
|--------------------------------------------------------------------------
|
| The unsubscribe link in teammate emails, answered without signing in.
| A mail app's one-click button posts to it as RFC 8058 describes and
| the email stops; a person who opens it lands on the page that turns
| it off and offers it back.
|
*/

package web

import (
	"net/url"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/notifications"
)

/*
NotificationLinkController answers the unsubscribe link.
*/
type NotificationLinkController struct {
	Unsubscribes *notifications.Unsubscribes
}

/*
Page opens the unsubscribe page for the link.
*/
func (c NotificationLinkController) Page(w fhttp.ResponseWriter, r *fhttp.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Location", "/email-preferences?n="+url.QueryEscape(r.PathValue("token")))
	w.WriteHeader(fhttp.StatusFound)
}

/*
OneClick unsubscribes as a mail app's one-click button asks.
*/
func (c NotificationLinkController) OneClick(w fhttp.ResponseWriter, r *fhttp.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	if _, err := c.Unsubscribes.Unsubscribe(r.Context(), r.PathValue("token")); err != nil {
		w.WriteHeader(fhttp.StatusNotFound)

		return
	}

	w.WriteHeader(fhttp.StatusOK)
}
