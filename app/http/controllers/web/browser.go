/*
|--------------------------------------------------------------------------
| Browser Responses
|--------------------------------------------------------------------------
|
| Pages a browser opens directly answer with redirects and cookies
| rather than JSON. A failure the person can act on lands on a page of
| the dashboard with its error code in the query, where the page
| explains it; everything else is an error response like any other.
|
*/

package web

import (
	"net/url"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	appauth "github.com/dimmihq/dimmi/app/auth"
)

/*
redirect sends the browser to the location.
*/
func redirect(w fhttp.ResponseWriter, location string) {
	w.Header().Set("Location", location)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.WriteHeader(fhttp.StatusFound)
}

/*
setCookie adds the cookie to the response.
*/
func setCookie(w fhttp.ResponseWriter, cookie fhttp.Cookie) {
	w.Header().Add("Set-Cookie", cookie.String())
}

/*
withError returns the page with the error code in its query.
*/
func withError(page, code string) string {
	return page + "?" + url.Values{"error": {code}}.Encode()
}

/*
clientOf describes the browser the request came from.
*/
func clientOf(r *fhttp.Request, cookies appauth.Cookies) appauth.Client {
	return appauth.Client{IP: fhttp.ClientIP(r), UserAgent: r.UserAgent(), DeviceToken: cookies.DeviceToken(r)}
}
