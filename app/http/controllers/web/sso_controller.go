/*
|--------------------------------------------------------------------------
| Single Sign-On Controller
|--------------------------------------------------------------------------
|
| The pages a company's identity provider sees. GET
| /auth/sso/{connection}/redirect sends a person there; POST
| /auth/saml/{connection}/acs takes the SAML answer it posts back from
| its own site, so the request checks other forms rely on are replaced
| by the attempt, its cookie and the request id the answer must name.
| GET /auth/saml/{connection}/metadata describes this side for the
| identity provider. A failure never shows why: the owners read that on
| the connection.
|
*/

package web

import (
	"cmp"
	"errors"

	"github.com/dimmihq/dimmi/framework/fault"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/ssologin"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ssoSettingsPage is where an owner's test sign-in lands.
*/
const ssoSettingsPage = "/d/settings/single-sign-on"

/*
maxAnswerBytes caps the form an identity provider posts, well above any
real SAML answer.
*/
const maxAnswerBytes = 512 << 10

/*
SSOController serves the single sign-on pages.
*/
type SSOController struct {
	Start   *ssologin.StartSignIn
	SAML    *ssologin.ConsumeSAML
	Shows   *ssologin.Metadata
	IDs     *ids.Registry
	Cookies appauth.Cookies
	Errors  fhttp.Errors
}

/*
Redirect sends the browser to the connection's identity provider to
sign in.
*/
func (c *SSOController) Redirect(w fhttp.ResponseWriter, r *fhttp.Request) {
	id, ok := c.connection(r)
	if !ok {
		c.Errors.Render(w, r, exceptions.ErrNotFound)

		return
	}

	returnTo := ""
	if asked := r.URL.Query().Get("redirect"); asked != "" {
		returnTo = appauth.SafeRedirect(asked)
	}

	started, err := c.Start.Handle(r.Context(), ssologin.StartSignInInput{
		ConnectionID: id, Intent: appauth.SSOIntentLogin, ReturnTo: returnTo,
	})
	if errors.Is(err, exceptions.ErrNotFound) {
		c.Errors.Render(w, r, err)

		return
	}

	if err != nil {
		redirect(w, withError("/login", cmp.Or(fault.CodeOf(err), "sso_failed")))

		return
	}

	setCookie(w, c.Cookies.SSOCookie(started.Browser, appauth.SSOAttemptLifetime))
	redirect(w, started.URL)
}

/*
ACS takes the SAML answer the identity provider posts back.
*/
func (c *SSOController) ACS(w fhttp.ResponseWriter, r *fhttp.Request) {
	r.Body = fhttp.MaxBytesReader(w, r.Body, maxAnswerBytes)

	id, ok := c.connection(r)
	response, relay := r.PostFormValue("SAMLResponse"), r.PostFormValue("RelayState")
	if !ok || response == "" {
		seeOther(w, withError("/login", "sso_failed"))

		return
	}

	browser := ""
	if cookie, err := r.Cookie(c.Cookies.SSOName()); err == nil {
		browser = cookie.Value
	}

	setCookie(w, c.Cookies.SSOCookie("", 0))

	result, err := c.SAML.Handle(r.Context(), ssologin.ConsumeSAMLInput{
		ConnectionID: id, Response: response, RelayState: relay,
		Browser: browser, Client: clientOf(r, c.Cookies),
	})
	if err != nil {
		seeOther(w, withError("/login", cmp.Or(fault.CodeOf(err), "sso_failed")))

		return
	}

	c.land(w, r, result)
}

/*
Metadata answers with this side's SAML metadata.
*/
func (c *SSOController) Metadata(w fhttp.ResponseWriter, r *fhttp.Request) {
	id, ok := c.connection(r)
	if !ok {
		c.Errors.Render(w, r, exceptions.ErrNotFound)

		return
	}

	document, err := c.Shows.Handle(r.Context(), id)
	if err != nil {
		c.Errors.Render(w, r, err)

		return
	}

	w.Header().Set("Content-Type", "application/samlmetadata+xml")
	w.WriteHeader(fhttp.StatusOK)
	_, _ = w.Write(document)
}

/*
land sends the browser where the answer led: a fresh start for an
answer nobody asked for, the settings page for a test, the challenge or
the app for a sign-in.
*/
func (c *SSOController) land(w fhttp.ResponseWriter, r *fhttp.Request, result ssologin.Result) {
	switch {
	case result.Restart:
		seeOther(w, "/auth/sso/"+r.PathValue("connection")+"/redirect")
	case result.Tested:
		seeOther(w, ssoSettingsPage+"?tested=1")
	default:
		if token := result.SignedIn.DeviceToken; token != "" {
			setCookie(w, c.Cookies.DeviceCookie(token))
		}

		seeOther(w, cmp.Or(result.ReturnTo, "/d"))
	}
}

/*
connection decodes the connection the path names.
*/
func (c *SSOController) connection(r *fhttp.Request) (ids.UUID, bool) {
	id, err := c.IDs.Decode(string(models.PrefixSSOConnection), r.PathValue("connection"))

	return id, err == nil
}

/*
seeOther sends the browser to the location with a GET, as the answer to
a form post.
*/
func seeOther(w fhttp.ResponseWriter, location string) {
	w.Header().Set("Location", location)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.WriteHeader(fhttp.StatusSeeOther)
}
