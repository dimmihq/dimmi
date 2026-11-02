/*
|--------------------------------------------------------------------------
| Request Locale
|--------------------------------------------------------------------------
|
| Every response is written in one locale, chosen once per request: the
| language the user or tenant picked, when the application knows it and
| serves it, then the best match for the browser's Accept-Language, and
| finally the default locale. The choice goes into the context with the
| translator, so handlers, views and error responses all read the same
| language, and into Content-Language, with Vary telling caches that the
| answer depends on Accept-Language.
|
| A preference usually belongs to the signed-in user, who is only known
| after authentication, so the middleware may run twice: once for every
| request and again after "auth", where the preference can be read.
|
*/

package middleware

import (
	"net/http"
	"slices"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/i18n"
)

/*
LocalePreference returns the locale the user or tenant behind a request
chose, or "" when there is none or it is not known yet.
*/
type LocalePreference func(r *http.Request) string

/*
Locale picks the locale of every request from the preference, which may
be nil, then the Accept-Language header, then the translator's default
locale, and serves the request in it.
*/
func Locale(t *i18n.Translator, prefer LocalePreference) fhttp.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			locale := pickLocale(t, prefer, r)

			w.Header().Set("Content-Language", locale)
			if !slices.Contains(w.Header().Values("Vary"), "Accept-Language") {
				w.Header().Add("Vary", "Accept-Language")
			}

			ctx := i18n.WithTranslator(i18n.WithLocale(r.Context(), locale), t)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

/*
pickLocale returns the preferred locale when the translator serves one
close to it, and the best match for Accept-Language otherwise.
*/
func pickLocale(t *i18n.Translator, prefer LocalePreference, r *http.Request) string {
	if prefer != nil {
		if locale, ok := t.Match(prefer(r)); ok {
			return locale
		}
	}

	return t.Negotiate(r.Header.Get("Accept-Language"))
}
