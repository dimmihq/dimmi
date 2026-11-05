/*
|--------------------------------------------------------------------------
| Locale Preference
|--------------------------------------------------------------------------
|
| A signed-in person reads the app in the language they picked, else in
| their workspace's language, else in the best match for their browser.
| Only languages the server serves count: a choice that stopped being
| served falls through to the next one instead of failing the request.
| The browser app follows the same order, so the page and the messages
| the server writes always agree.
|
*/

package middleware

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/middleware"
	"github.com/dimmihq/dimmi/framework/i18n"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
)

/*
LocalePreference returns the preference the request locale middleware
asks once the caller, and the workspace when the route has one, are
known: the caller's own language, then the workspace's, when served.
*/
func LocalePreference(t *i18n.Translator) middleware.LocalePreference {
	return func(r *fhttp.Request) string {
		var user *string
		if p, ok := appauth.CurrentPrincipal(r.Context()); ok {
			user = p.User.Locale
		}

		var workspace *string
		if m, ok := authz.CurrentMembership(r.Context()); ok {
			workspace = &m.Workspace.Locale
		}

		locale, _ := appauth.PreferredLocale(t, user, workspace)

		return locale
	}
}
