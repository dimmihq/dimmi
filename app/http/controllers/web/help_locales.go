/*
|--------------------------------------------------------------------------
| Help Center Languages
|--------------------------------------------------------------------------
|
| A help center's default language lives at its root, as it always did,
| so no address it ever had stops working. Every other language lives
| under its own prefix: /fr/articles/<french slug>. The prefix is read
| off before routing, so every page of the help center, its forms and
| files included, answers under it the same way. A prefix for the
| default language leads back to the plain address, and one the help
| center does not offer finds nothing.
|
| The first time a reader opens the home page, the languages their
| browser asks for pick one the help center offers. A reader who picks
| a language from the menu keeps it, and is never moved off a page they
| were linked to.
|
*/

package web

import (
	"context"
	"net/url"
	"strings"
	"time"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/resources/views/helpcenter"
)

/*
languageCookie keeps the language a reader picked.
*/
const languageCookie = "dimmi_hc_lang"

/*
readingKey marks a request read in a language other than the default,
with the path it arrived at.
*/
type readingKey struct{}

/*
reading is the language a request names and the path it arrived at.
*/
type reading struct {
	locale string
	path   string
}

/*
stripLocale reads a language prefix off a help center path, leaving the
path every page is routed by.
*/
func stripLocale(next fhttp.Handler) fhttp.Handler {
	return fhttp.HandlerFunc(func(w fhttp.ResponseWriter, r *fhttp.Request) {
		rest, ok := strings.CutPrefix(r.URL.Path, "/hc/")
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		center, after, _ := strings.Cut(rest, "/")
		seg, tail, _ := strings.Cut(after, "/")
		locale, found := helpcenters.LocaleOfSegment(seg)
		if !found || center == "" {
			next.ServeHTTP(w, r)
			return
		}
		read := r.Clone(context.WithValue(r.Context(), readingKey{}, reading{locale: locale, path: r.URL.Path}))
		read.URL.Path = strings.TrimSuffix("/hc/"+center+"/"+tail, "/")
		read.URL.RawPath = ""
		next.ServeHTTP(w, read)
	})
}

/*
readingLocale is the language the request's address names, empty for
the default.
*/
func readingLocale(ctx context.Context) string {
	read, _ := ctx.Value(readingKey{}).(reading)
	return read.locale
}

/*
arrivedPath is the path the request arrived at, its language prefix
included.
*/
func arrivedPath(r *fhttp.Request) string {
	if read, ok := r.Context().Value(readingKey{}).(reading); ok {
		return read.path
	}
	return r.URL.Path
}

/*
localeAnswered handles an address that names a language the help center
cannot be read in under a prefix: its default, which leads back to the
plain address, or one it does not offer, which finds nothing. It
reports whether it answered.
*/
func (c *HelpCenterController) localeAnswered(w fhttp.ResponseWriter, r *fhttp.Request, h models.HelpCenter,
	base string,
) bool {
	locale := readingLocale(r.Context())
	switch {
	case locale == "":
		return false
	case locale == h.Locale:
		to := base + strings.TrimPrefix(r.URL.Path, "/hc/"+r.PathValue("center"))
		if to == "" {
			to = "/"
		}
		if r.URL.RawQuery != "" {
			to += "?" + r.URL.RawQuery
		}
		w.Header().Set("Location", to)
		w.WriteHeader(fhttp.StatusMovedPermanently)
		return true
	case !helpcenters.OffersLocale(h, locale):
		w.WriteHeader(fhttp.StatusNotFound)
		return true
	}
	return false
}

/*
prefixOf is the path prefix of a language of the help center, empty for
its default.
*/
func prefixOf(h models.HelpCenter, locale string) string {
	if locale == "" || locale == h.Locale {
		return ""
	}
	return "/" + locale
}

/*
pageIn gives a page's path in a language, and whether it exists there.
*/
type pageIn func(locale string) (string, bool)

/*
samePage is a page that reads the same in every language, such as the
changelog: the language picker keeps the reader on it.
*/
func samePage(path string) pageIn {
	return func(string) (string, bool) { return path, true }
}

/*
localize gives the page its language, its language picker and, for a
page search engines should know in every language, its alternates.
Root is the help center's canonical address, base its address here.
*/
func localize(center *helpcenter.Center, snap helpcenters.Snapshot, at pageAt, page pageIn, alternates bool) {
	h := snap.HelpCenter
	center.Locale, center.Languages, center.Alternates = snap.Locale, nil, nil
	locales := helpcenters.LocalesOf(h)
	if len(locales) < 2 {
		return
	}
	for _, l := range locales {
		path, ok := page(l)
		if !ok {
			path = "/"
		}
		to := at.base + prefixOf(h, l) + path
		if path == "/" {
			to = strings.TrimSuffix(to, "/")
			if to == "" {
				to = "/"
			}
		}
		lang := helpcenters.LanguageOf(l)
		center.Languages = append(center.Languages, helpcenter.LanguageLink{
			Code: l, Name: lang.Native, URL: to + "?lang=" + url.QueryEscape(l), Current: l == snap.Locale,
		})
		if alternates && ok {
			center.Alternates = append(center.Alternates, helpcenter.Alternate{
				Lang: l, URL: strings.TrimSuffix(at.root+prefixOf(h, l)+path, "/"),
			})
			if l == h.Locale {
				center.Alternates = append(center.Alternates, helpcenter.Alternate{
					Lang: "x-default", URL: strings.TrimSuffix(at.root+path, "/"),
				})
			}
		}
	}
}

/*
pageAt is where a help center answers: its canonical root and the base
this request reached it under.
*/
type pageAt struct {
	root string
	base string
}

/*
keepLanguage remembers the language a reader picked from the menu, so
the home page opens in it next time.
*/
func keepLanguage(w fhttp.ResponseWriter, r *fhttp.Request, h models.HelpCenter, base string) {
	picked := r.URL.Query().Get("lang")
	if picked == "" || !helpcenters.OffersLocale(h, picked) {
		return
	}
	cookie := fhttp.Cookie{
		Name: languageCookie, Value: picked, Path: cookiePath(base), HttpOnly: true, Secure: r.TLS != nil,
		SameSite: fhttp.SameSiteLaxMode, Expires: time.Now().AddDate(1, 0, 0),
	}
	w.Header().Add("Set-Cookie", cookie.String())
}

/*
cookiePath is the path a help center's cookies belong to.
*/
func cookiePath(base string) string {
	if base == "" {
		return "/"
	}
	return base
}

/*
preferredLocale is the language the home page should open in for a
reader who did not name one: the one they picked before, else the first
their browser asks for that the help center offers, else its default.
*/
func preferredLocale(r *fhttp.Request, h models.HelpCenter) string {
	if picked := r.URL.Query().Get("lang"); picked != "" {
		return h.Locale
	}
	if cookie, err := r.Cookie(languageCookie); err == nil && helpcenters.OffersLocale(h, cookie.Value) {
		return cookie.Value
	}
	return helpcenters.PickLocale(r.Header.Get("Accept-Language"), helpcenters.LocalesOf(h))
}
