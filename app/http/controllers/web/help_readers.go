/*
|--------------------------------------------------------------------------
| Help Center Readers
|--------------------------------------------------------------------------
|
| A help center with a login address offers its readers a Sign in
| button: it leads to the team's product, which signs an identity token
| and sends the reader back to /sso with the token in the fragment. That
| page posts it here, and a signed, httpOnly cookie remembers the reader
| for an hour. A signed-in reader's pages show what their audiences open
| and are never stored by any cache. A reader goes back only to a page of
| the same help center.
|
*/

package web

import (
	"net/url"
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/resources/views/helpcenter"
)

/*
readerCookie remembers a signed-in reader.
*/
const readerCookie = "dimmi_help_reader"

/*
realSite is a real help center's frame and pages for the request: the
signed-in reader's when the cookie names one, anyone's otherwise.
*/
func (c *HelpCenterController) realSite(w fhttp.ResponseWriter, r *fhttp.Request, snap helpcenters.Snapshot,
	base string,
) (helpcenter.Center, HelpLibrary, bool) {
	center := centerOf(snap.HelpCenter, base)
	center.Origin = origin(r)

	contact, signed := c.readerOf(r, snap.HelpCenter)
	if !signed {
		center.SignInURL = signInURL(snap.HelpCenter, center, r.URL.Path)
		return center, newSite(r.Context(), c.Public, snap, nil), true
	}

	mine, err := c.Public.LoadFor(r.Context(), r.PathValue("center"), contact)
	if err != nil {
		c.Errors.Render(w, r, err)
		return center, nil, false
	}

	w.Header().Set("Cache-Control", "private, no-store")
	center.SignedIn, center.SignOutURL = true, base+"/sso/sign-out"

	return center, newSite(r.Context(), c.Public, mine, &contact), true
}

/*
readerOf returns the reader the request's cookie names for the help
center.
*/
func (c *HelpCenterController) readerOf(r *fhttp.Request, h models.HelpCenter) (ids.UUID, bool) {
	cookie, err := r.Cookie(readerCookie)
	if err != nil || c.Readers == nil {
		return ids.Nil(), false
	}

	return c.Readers.Reader(h, cookie.Value)
}

/*
signInURL leads to the team's login address, which sends the reader back
to /sso and then to the page they were on. Empty when the help center
takes no sign-in.
*/
func signInURL(h models.HelpCenter, center helpcenter.Center, path string) string {
	login := helpcenters.SettingsOf(h).LoginURL
	if login == "" {
		return ""
	}

	back := center.Origin + center.Base + "/sso?next=" + url.QueryEscape(path)
	sep := "?"
	if strings.Contains(login, "?") {
		sep = "&"
	}

	return login + sep + "return_to=" + url.QueryEscape(back)
}

/*
SignInPage takes the reader back from the team's product.
*/
func (c *HelpCenterController) SignInPage(w fhttp.ResponseWriter, r *fhttp.Request) {
	center, _, ok := c.site(w, r)
	if !ok {
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	page := helpcenter.ReaderSignInPage(center, center.Base+"/sso", r.URL.Query().Get("next"), c.styles())
	c.respond(w, r, fhttp.StatusOK, page)
}

/*
SignIn checks the token, remembers the reader and sends them back to the
page they were on.
*/
func (c *HelpCenterController) SignIn(w fhttp.ResponseWriter, r *fhttp.Request) {
	snap, base, ok := c.helpCenter(w, r)
	if !ok {
		return
	}

	value, until, err := c.Readers.SignIn(r.Context(), snap.HelpCenter, r.FormValue("token"))
	if err != nil {
		c.Errors.Render(w, r, err)
		return
	}

	cookie := c.cookie(r, base)
	cookie.Value, cookie.Expires = value, until
	w.Header().Add("Set-Cookie", cookie.String())
	back(w, nextPage(base, r.FormValue("next")))
}

/*
SignOut forgets the reader.
*/
func (c *HelpCenterController) SignOut(w fhttp.ResponseWriter, r *fhttp.Request) {
	_, base, ok := c.helpCenter(w, r)
	if !ok {
		return
	}

	cookie := c.cookie(r, base)
	cookie.MaxAge = -1
	w.Header().Add("Set-Cookie", cookie.String())
	back(w, nextPage(base, ""))
}

/*
helpCenter finds the real help center the request names, and the path
its pages live under.
*/
func (c *HelpCenterController) helpCenter(w fhttp.ResponseWriter, r *fhttp.Request,
) (helpcenters.Snapshot, string, bool) {
	base := "/hc/" + r.PathValue("center")
	if hostedHere(r.Context()) {
		base = ""
	}

	if c.Public == nil || c.Readers == nil {
		w.WriteHeader(fhttp.StatusNotFound)
		return helpcenters.Snapshot{}, base, false
	}

	snap, err := c.Public.Load(r.Context(), r.PathValue("center"))
	if err != nil {
		w.WriteHeader(fhttp.StatusNotFound)
		return snap, base, false
	}

	return snap, base, true
}

/*
cookie is the reader cookie for the help center's pages.
*/
func (c *HelpCenterController) cookie(r *fhttp.Request, base string) *fhttp.Cookie {
	path := base
	if path == "" {
		path = "/"
	}

	return &fhttp.Cookie{
		Name: readerCookie, Path: path, HttpOnly: true, Secure: r.TLS != nil, SameSite: fhttp.SameSiteLaxMode,
	}
}

/*
nextPage is where a reader goes after signing in or out: a page of this
help center, else its home.
*/
func nextPage(base, next string) string {
	home := base
	if home == "" {
		home = "/"
	}

	if next == "" || !strings.HasPrefix(next, home) || strings.HasPrefix(next, "//") || strings.Contains(next, "\\") {
		return home
	}

	return next
}
