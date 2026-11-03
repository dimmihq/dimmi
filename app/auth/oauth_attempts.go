/*
|--------------------------------------------------------------------------
| Provider Sign-In Attempts
|--------------------------------------------------------------------------
|
| A sign-in at an identity provider leaves the site and comes back. What
| the browser asked for on the way out, signing in, linking an account
| or signing in afresh, the invitation it carried and where to land
| afterwards, travels in a cookie sealed with a key derived from the app
| key: the browser holds it but can neither read nor change it. The
| cookie also holds the state the provider must hand back, so a callback
| opened in another browser fails, and the user a link or fresh sign-in
| was started for, so a session swapped in between gains nothing.
|
| Landing pages are relative paths on this site only. Anything else,
| another host, a protocol-relative "//host" or a backslash browsers
| read as a slash, lands on the home page instead.
|
*/

package auth

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/ids"
)

/*
What a sign-in at a provider is for: signing in, linking the provider's
account to the signed-in user, or signing that user in afresh.
*/
const (
	IntentLogin  = "login"
	IntentLink   = "link"
	IntentReauth = "reauth"
)

/*
OAuthAttemptLifetime is how long a browser has to come back from the
provider.
*/
const OAuthAttemptLifetime = 10 * time.Minute

/*
OAuthKeyPurpose names the key derived from the app key that seals
attempts.
*/
const OAuthKeyPurpose = "oauth-attempts"

/*
OAuthAttempt is a sign-in at a provider in progress. Provider is the
provider's key and State what it must hand back. UserID is the
signed-in user a link or fresh sign-in was started for, else nil.
Invitation is the invitation token the browser carried, and Redirect
the safe page to land on.
*/
type OAuthAttempt struct {
	Provider   string   `json:"provider"`
	State      string   `json:"state"`
	Intent     string   `json:"intent"`
	UserID     ids.UUID `json:"user_id"`
	Invitation string   `json:"invitation,omitempty"`
	Redirect   string   `json:"redirect"`
}

/*
OAuthAttempts seals attempts into cookie values and opens them again.
*/
type OAuthAttempts struct {
	Encrypter *encryption.AppEncrypter
}

/*
Seal returns the cookie value of the attempt.
*/
func (a OAuthAttempts) Seal(attempt OAuthAttempt) (string, error) {
	plain, _ := json.Marshal(attempt)

	return a.Encrypter.EncryptString(string(plain))
}

/*
Open returns the attempt sealed in the cookie value, and false for a
value this server did not seal.
*/
func (a OAuthAttempts) Open(value string) (OAuthAttempt, bool) {
	var attempt OAuthAttempt

	plain, err := a.Encrypter.DecryptString(value)
	if err != nil || json.Unmarshal([]byte(plain), &attempt) != nil {
		return OAuthAttempt{}, false
	}

	return attempt, true
}

/*
SafeRedirect returns the path when it stays on this site, else "/".
*/
func SafeRedirect(path string) string {
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") || strings.ContainsAny(path, "\\\r\n") {
		return "/"
	}

	return path
}
