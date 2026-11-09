/*
|--------------------------------------------------------------------------
| Help Center Email Sign-in
|--------------------------------------------------------------------------
|
| A reader signs in with their email: one page asks for the address and
| mails a code, the next takes the code. When the help center also takes
| the team's own login, the first page offers it above the email. Every
| mistake comes back on the same page, in words, with what the reader
| typed still there; these pages are never cached.
|
*/

package web

import (
	"errors"
	"net/url"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/fault"
	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/actions/widget"
	"github.com/dimmihq/dimmi/resources/views/helpcenter"
)

/*
EmailSignIn shows the email sign-in page, or sends the reader to the
team's login when the help center takes only that.
*/
func (c *HelpCenterController) EmailSignIn(w fhttp.ResponseWriter, r *fhttp.Request) {
	snap, base, ok := c.helpCenter(w, r)
	if !ok {
		return
	}

	center := c.readerCenter(r, snap, base)
	f := readerForm(center, snap, nextPage(base, r.URL.Query().Get("next")))
	if _, _, signed := c.readerOf(r, snap.HelpCenter); signed {
		back(w, f.Next)
		return
	}

	if !helpcenters.SettingsOf(snap.HelpCenter).EmailSignInOn() {
		c.productOnly(w, r, f.ProductURL)
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	c.respond(w, r, fhttp.StatusOK, helpcenter.ReaderEmailPage(center, f, c.styles()))
}

/*
productOnly sends the reader to the team's login, the only way in, or
says the help center takes no sign-in.
*/
func (c *HelpCenterController) productOnly(w fhttp.ResponseWriter, r *fhttp.Request, product string) {
	if product == "" {
		c.NotFound(w, r)
		return
	}

	back(w, product)
}

/*
SendCode mails a code to the address and shows the page that takes it.
*/
func (c *HelpCenterController) SendCode(w fhttp.ResponseWriter, r *fhttp.Request) {
	snap, base, ok := c.helpCenter(w, r)
	if !ok {
		return
	}

	center := c.readerCenter(r, snap, base)
	f := readerForm(center, snap, nextPage(base, r.FormValue("next")))
	f.Email = strings.TrimSpace(r.FormValue("email"))
	w.Header().Set("Cache-Control", "no-store")

	sentTo, err := c.Readers.SendCode(r.Context(), helpcenters.CodeRequest{
		Center: snap.HelpCenter, Email: f.Email, Network: network(r),
	})
	if err == nil {
		f.Masked = sentTo
		c.respond(w, r, fhttp.StatusOK, helpcenter.ReaderCodePage(center, f, c.styles()))

		return
	}

	status, message, shown := readerProblem(err)
	if !shown {
		c.Errors.Render(w, r, err)
		return
	}

	f.Error = message
	if r.FormValue("resend") != "" {
		f.Masked = masked(f.Email)
		c.respond(w, r, status, helpcenter.ReaderCodePage(center, f, c.styles()))

		return
	}

	c.respond(w, r, status, helpcenter.ReaderEmailPage(center, f, c.styles()))
}

/*
CheckCode signs the reader in with the code and sends them back to the
page they were on.
*/
func (c *HelpCenterController) CheckCode(w fhttp.ResponseWriter, r *fhttp.Request) {
	snap, base, ok := c.helpCenter(w, r)
	if !ok {
		return
	}

	center := c.readerCenter(r, snap, base)
	f := readerForm(center, snap, nextPage(base, r.FormValue("next")))
	f.Email = strings.TrimSpace(r.FormValue("email"))

	session, err := c.Readers.CheckCode(r.Context(), snap.HelpCenter, f.Email, r.FormValue("code"))
	if err == nil {
		c.remember(w, r, base, session)
		back(w, f.Next)

		return
	}

	status, message, shown := readerProblem(err)
	if !shown {
		c.Errors.Render(w, r, err)
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	f.Error, f.Masked = message, masked(f.Email)
	c.respond(w, r, status, helpcenter.ReaderCodePage(center, f, c.styles()))
}

/*
readerCenter is the frame of the sign-in pages, which show no reader but
the same sections in the header as every other page.
*/
func (c *HelpCenterController) readerCenter(r *fhttp.Request, snap helpcenters.Snapshot, base string,
) helpcenter.Center {
	center := centerOf(snap.HelpCenter, base)
	center.Origin = origin(r)
	c.withMessenger(&center, snap)
	c.announce(r, snap.HelpCenter, &center)

	return center
}

/*
readerForm is the sign-in pages' form: where each form goes, the page
to return to and, when the help center takes it, the team's login.
*/
func readerForm(center helpcenter.Center, snap helpcenters.Snapshot, next string) helpcenter.ReaderForm {
	login := helpcenters.SettingsOf(snap.HelpCenter).LoginURL

	return helpcenter.ReaderForm{
		Action: center.Base + "/sso/codes", CheckURL: center.Base + "/sso/codes/check",
		ResendURL:  center.Base + "/sso/codes",
		RestartURL: center.Base + "/sso/sign-in?next=" + url.QueryEscape(next), Next: next,
		ProductURL: productLogin(login, center, next),
	}
}

/*
readerProblem is what to tell a reader about a mistake on the sign-in
pages, and the status to answer with; false for a failure that is not
theirs.
*/
func readerProblem(err error) (int, string, bool) {
	var f *fault.Error
	if !errors.As(err, &f) {
		return 0, "", false
	}

	if f.Kind() == fault.RateLimited {
		return fhttp.StatusTooManyRequests, f.Message(), true
	}

	shown := []fault.Kind{fault.Invalid, fault.Gone, fault.NotFound, fault.Unauthenticated}

	return fhttp.StatusUnprocessableEntity, f.Message(), slices.Contains(shown, f.Kind())
}

/*
network is the part of the reader's address codes are throttled by,
empty when unknown.
*/
func network(r *fhttp.Request) string {
	prefix := widget.IPPrefix(fhttp.ClientIP(r))
	if prefix == nil {
		return ""
	}

	return prefix.String()
}

/*
masked is the address as the code page repeats it.
*/
func masked(email string) string {
	if normal, ok := widget.NormalizeTypedEmail(email); ok {
		return widget.MaskEmail(normal)
	}

	return widget.MaskEmail(email)
}
