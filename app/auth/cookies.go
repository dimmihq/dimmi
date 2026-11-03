/*
|--------------------------------------------------------------------------
| Browser Cookies
|--------------------------------------------------------------------------
|
| Cookies the product sets besides the session: the device cookie, kept
| for a year so a browser is recognized across sign-ins, and the
| attempt cookie, which ties a sign-in at an identity provider to the
| browser that started it for the few minutes it takes, and the
| challenge cookie, which holds a sign-in waiting on its second factor
| for as long as the challenge lasts. Served over
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
oauthCookie is the name of the attempt cookie, before its prefix.
*/
const oauthCookie = "dimmi_oauth"

/*
twoFactorCookie is the name of the challenge cookie, before its prefix.
*/
const twoFactorCookie = "dimmi_2fa"

/*
Cookies names and builds the product's own cookies. Device names the
device cookie. Secure is false on an install served over plain HTTP.
*/
type Cookies struct {
	Device string
	Secure bool
}

/*
DeviceName returns the name of the device cookie, with its prefix.
*/
func (c Cookies) DeviceName() string {
	return c.prefixed(c.Device)
}

/*
OAuthName returns the name of the attempt cookie, with its prefix.
*/
func (c Cookies) OAuthName() string {
	return c.prefixed(oauthCookie)
}

/*
OAuthCookie returns the cookie that keeps a sign-in attempt's sealed
value for the lifetime, or deletes it with a zero lifetime. It is sent
back on the provider's top-level redirect, never on a request another
site makes in the background.
*/
func (c Cookies) OAuthCookie(value string, lifetime time.Duration) fhttp.Cookie {
	maxAge := int(lifetime.Seconds())
	if lifetime <= 0 {
		maxAge = -1
	}

	return fhttp.Cookie{
		Name:     c.OAuthName(),
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		Secure:   c.Secure,
		HttpOnly: true,
		SameSite: fhttp.SameSiteLaxMode,
	}
}

/*
TwoFactorName returns the name of the challenge cookie, with its prefix.
*/
func (c Cookies) TwoFactorName() string {
	return c.prefixed(twoFactorCookie)
}

/*
TwoFactorCookie returns the cookie that holds a second-factor challenge
for the lifetime, or deletes it with a zero lifetime. Scripts never
read it: the browser only sends it back when answering.
*/
func (c Cookies) TwoFactorCookie(value string, lifetime time.Duration) fhttp.Cookie {
	cookie := c.OAuthCookie(value, lifetime)
	cookie.Name = c.TwoFactorName()

	return cookie
}

/*
TwoFactorToken returns the request's challenge cookie value, or "".
*/
func (c Cookies) TwoFactorToken(r *fhttp.Request) string {
	cookie, err := r.Cookie(c.TwoFactorName())
	if err != nil {
		return ""
	}

	return cookie.Value
}

func (c Cookies) prefixed(name string) string {
	if c.Secure {
		return "__Host-" + name
	}

	return name
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
