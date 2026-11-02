/*
|--------------------------------------------------------------------------
| Browser Cookies
|--------------------------------------------------------------------------
|
| Cookies the product sets besides the session: the device cookie, kept
| for a year so a browser is recognized across sign-ins. Served over
| HTTPS a cookie gets the __Host- prefix, which browsers only accept on
| a Secure cookie for the whole site; a self-hosted install on plain
| HTTP drops the prefix and the Secure flag, or browsers would refuse
| it.
|
*/

package auth

import (
	"time"

	fhttp "github.com/dimmihq/dimmi/framework/http"
)

/*
deviceCookieLifetime is how long a browser keeps its device cookie.
*/
const deviceCookieLifetime = 365 * 24 * time.Hour

/*
Cookies names and builds the product's own cookies. Secure is false on
an install served over plain HTTP.
*/
type Cookies struct {
	Device string
	Secure bool
}

/*
DeviceName returns the name of the device cookie, with its prefix.
*/
func (c Cookies) DeviceName() string {
	if c.Secure {
		return "__Host-" + c.Device
	}

	return c.Device
}

/*
DeviceToken returns the request's device cookie value, or "".
*/
func (c Cookies) DeviceToken(r *fhttp.Request) string {
	cookie, err := r.Cookie(c.DeviceName())
	if err != nil {
		return ""
	}

	return cookie.Value
}

/*
DeviceCookie returns the cookie that hands a browser its device token.
*/
func (c Cookies) DeviceCookie(token string) fhttp.Cookie {
	return fhttp.Cookie{
		Name:     c.DeviceName(),
		Value:    token,
		Path:     "/",
		MaxAge:   int(deviceCookieLifetime.Seconds()),
		Secure:   c.Secure,
		HttpOnly: true,
		SameSite: fhttp.SameSiteLaxMode,
	}
}
