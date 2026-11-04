/*
|--------------------------------------------------------------------------
| Account Links
|--------------------------------------------------------------------------
|
| The pages mail sends people to. A secret always rides after the # of
| the address: browsers never send that part to any server, so it never
| reaches a log, a proxy or a Referer header, and a mail scanner that
| opens the page cannot use it, because only the page's script sends it
| on, in the body of a request.
|
*/

package auth

import (
	"strings"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Links builds the addresses of the account pages under Base, the
application's URL.
*/
type Links struct {
	Base string
}

/*
VerifyEmail is the page that proves an email address.
*/
func (l Links) VerifyEmail(id ids.UUID, token string) string {
	return l.page("/verify-email") + "#id=" + models.PublicID(models.PrefixOneTimeToken, id) + "&token=" + token
}

/*
MagicLink is the page that signs in with a mailed link.
*/
func (l Links) MagicLink(token string) string {
	return l.page("/login/magic") + "#token=" + token
}

/*
ResetPassword is the page that chooses a new password.
*/
func (l Links) ResetPassword(id ids.UUID, token string) string {
	return l.page("/reset-password") + "#id=" + models.PublicID(models.PrefixPasswordReset, id) + "&token=" + token
}

/*
SecurityPage is the path of the dashboard page listing the account's
sessions and sign-in methods, where two-factor authentication is set
up. The dashboard lives under /d.
*/
const SecurityPage = "/d/settings/security"

/*
Security is the page listing the account's sessions and sign-in
methods.
*/
func (l Links) Security() string {
	return l.page(SecurityPage)
}

/*
Invitation is the page that accepts an invitation to a workspace.
*/
func (l Links) Invitation(token string) string {
	return l.page("/invitation") + "#token=" + token
}

/*
App is the dashboard's front page.
*/
func (l Links) App() string {
	return l.page("/d")
}

/*
Members is the dashboard page listing a workspace's members.
*/
func (l Links) Members() string {
	return l.page("/d/settings/members")
}

/*
Tokens is the page listing the user's personal access tokens.
*/
func (l Links) Tokens() string {
	return l.page("/d/settings/tokens")
}

func (l Links) page(path string) string {
	return strings.TrimRight(l.Base, "/") + path
}
