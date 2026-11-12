/*
|--------------------------------------------------------------------------
| Help Center Readers
|--------------------------------------------------------------------------
|
| A help center offers its readers a Sign in button when audiences keep
| some of it from them or when it has the team's login address. It leads
| to the email sign-in page, or straight to the team's product when that
| is the only way in; the product signs an identity token and sends the
| reader back to /sso with the token in the fragment, where the messenger
| also sends readers with a pass. That page posts either here, and a
| signed, httpOnly cookie remembers the reader for seven days, renewed as
| they come back. A signed-in reader's pages show what their audiences
| open and are never stored by any cache. A reader goes back only to a
| page of the same help center.
|
*/

package web

import (
	"net/url"
	"strings"
	"time"

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
	center.Canonical = c.canonical(r, snap.HelpCenter)
	c.withMessenger(&center, snap)

	updates := c.announce(r, snap.HelpCenter, &center)
	contact, until, signed := c.readerOf(r, snap.HelpCenter)
	if !signed {
		center.SignInURL = signInURL(snap, center, r.URL.Path)
		s := newSite(r.Context(), c.Public, snap, nil)
		s.updates = updates
		return center, s, true
	}

	if fresh, renew := c.Readers.Renewal(snap.HelpCenter, contact, until); renew {
		c.remember(w, r, base, fresh)
	}

	mine, err := c.Public.LoadFor(r.Context(), r.PathValue("center"), contact)
	if err != nil {
		c.Errors.Render(w, r, err)
		return center, nil, false
	}

	w.Header().Set("Cache-Control", "private, no-store")
	center.SignedIn, center.SignOutURL = true, base+"/sso/sign-out"

	s := newSite(r.Context(), c.Public, mine, &contact)
	s.updates = updates
	return center, s, true
}

/*
announce links the changelog from the help center's header once it has
an entry, and the feature requests once the board is open, and returns
the newest entries for the home page.
*/
func (c *HelpCenterController) announce(r *fhttp.Request, h models.HelpCenter, center *helpcenter.Center,
) []helpcenter.Update {
	if helpcenters.SettingsOf(h).FeedbackBoard {
		defer func() {
			center.Sections = append(center.Sections, helpcenter.Section{
				Key: "feedback", Name: "Feature requests", Icon: "lightbulb", Path: "/feedback",
			})
		}()
	}
	if c.Changelog == nil {
		return nil
	}
	latest, err := c.Changelog.Latest(r.Context(), h, 3)
	if err != nil || len(latest) == 0 {
		return nil
	}
	center.Sections = append(center.Sections, helpcenter.Section{
		Key: "changelog", Name: helpcenters.ChangelogOf(h).Title, Icon: "megaphone", Path: "/changelog",
	})
	out := make([]helpcenter.Update, len(latest))
	for i, e := range latest {
		out[i] = helpcenter.Update{
			Title: deref(e.PublishedTitle), Path: "/changelog/" + e.Slug, Published: *e.PublishedAt,
		}
		if len(e.Labels) > 0 {
			out[i].Label = e.Labels[0].Name
		}
	}
	return out
}

/*
readerOf returns the reader the request's cookie names for the help
center, and when the cookie ends.
*/
func (c *HelpCenterController) readerOf(r *fhttp.Request, h models.HelpCenter) (ids.UUID, time.Time, bool) {
	cookie, err := r.Cookie(readerCookie)
	if err != nil || c.Readers == nil {
		return ids.Nil(), time.Time{}, false
	}

	return c.Readers.Reader(h, cookie.Value)
}

/*
signInURL is where the Sign in button leads: the email sign-in page,
when the help center takes email and signing in could show more, else
the team's login. Empty when the help center offers no sign-in.
*/
func signInURL(snap helpcenters.Snapshot, center helpcenter.Center, path string) string {
	s := helpcenters.SettingsOf(snap.HelpCenter)
	if s.EmailSignInOn() && (snap.Restricted || s.LoginURL != "") {
		return center.Base + "/sso/sign-in?next=" + url.QueryEscape(path)
	}

	return productLogin(s.LoginURL, center, path)
}

/*
productLogin leads to the team's login address, which sends the reader
back to /sso and then to the page they were on. Empty without one.
*/
func productLogin(login string, center helpcenter.Center, path string) string {
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
SignIn checks the token or the messenger's pass, remembers the reader
and sends them back to the page they were on. A pass that no longer
works leads to the email sign-in page, saying so, when the help center
takes email.
*/
func (c *HelpCenterController) SignIn(w fhttp.ResponseWriter, r *fhttp.Request) {
	snap, base, ok := c.helpCenter(w, r)
	if !ok {
		return
	}

	next := nextPage(base, r.FormValue("next"))
	session, err := c.readerSignIn(r, snap)
	switch {
	case err == nil:
		c.remember(w, r, base, session)
		back(w, next)
	case r.FormValue("pass") != "" && helpcenters.SettingsOf(snap.HelpCenter).EmailSignInOn():
		center := c.readerCenter(r, snap, base)
		f := readerForm(center, snap, next)
		_, f.Error, _ = readerProblem(err)
		w.Header().Set("Cache-Control", "no-store")
		c.respond(w, r, fhttp.StatusUnauthorized, helpcenter.ReaderEmailPage(center, f, c.styles()))
	default:
		c.Errors.Render(w, r, err)
	}
}

/*
readerSignIn signs in the reader the form's pass or identity token
names.
*/
func (c *HelpCenterController) readerSignIn(r *fhttp.Request, snap helpcenters.Snapshot,
) (helpcenters.ReaderSession, error) {
	if pass := r.FormValue("pass"); pass != "" {
		return c.Readers.RedeemPass(r.Context(), snap.HelpCenter, pass)
	}

	return c.Readers.SignIn(r.Context(), snap.HelpCenter, r.FormValue("token"))
}

/*
remember sets the cookie that remembers a reader.
*/
func (c *HelpCenterController) remember(w fhttp.ResponseWriter, r *fhttp.Request, base string,
	s helpcenters.ReaderSession,
) {
	cookie := c.cookie(r, base)
	cookie.Value, cookie.Expires = s.Value, s.Until
	w.Header().Add("Set-Cookie", cookie.String())
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
