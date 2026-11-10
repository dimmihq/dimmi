/*
|--------------------------------------------------------------------------
| Slack Token Answer
|--------------------------------------------------------------------------
|
| oauth.v2.access answers with the bot token at the top level and the
| approver's token under authed_user; either may be missing, since an
| install may ask only for one kind. Everything else Slack says about
| the install is kept in Raw for the caller, with every token removed,
| so Raw is safe to store or log.
|
*/

package slack

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/oauthclient"
)

/*
The answer's fields that hold tokens, at the top level and under
authed_user.
*/
const (
	accessField  = "access_token"
	refreshField = "refresh_token"
)

/*
rawKeeper is an answer that keeps its raw fields.
*/
type rawKeeper interface {
	keep(body []byte) error
}

/*
accessAnswer is the answer of oauth.v2.access.
*/
type accessAnswer struct {
	OK           bool       `json:"ok"`
	Error        string     `json:"error"`
	AccessToken  string     `json:"access_token"`
	RefreshToken string     `json:"refresh_token"`
	ExpiresIn    int64      `json:"expires_in"`
	Scope        string     `json:"scope"`
	AppID        string     `json:"app_id"`
	Team         named      `json:"team"`
	AuthedUser   authedUser `json:"authed_user"`

	raw map[string]any
}

/*
named is a Slack object with an id and a name.
*/
type named struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

/*
authedUser is the person who approved the install, with their own
token when user scopes were asked for.
*/
type authedUser struct {
	ID           string `json:"id"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	Scope        string `json:"scope"`
}

/*
keep reads the raw fields of the answer, without its tokens.
*/
func (a *accessAnswer) keep(body []byte) error {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return fmt.Errorf("%w: decode oauth.v2.access: %w", oauthclient.ErrProvider, err)
	}

	delete(raw, accessField)
	delete(raw, refreshField)

	if user, ok := raw["authed_user"].(map[string]any); ok {
		delete(user, accessField)
		delete(user, refreshField)
	}

	a.raw = raw

	return nil
}

/*
grant normalizes the answer. Expiries count from now, when the answer
was read.
*/
func (a *accessAnswer) grant(now time.Time) oauthclient.Grant {
	return oauthclient.Grant{
		Scopes: scopes(a.Scope), UserScopes: scopes(a.AuthedUser.Scope), UserID: a.AuthedUser.ID,
		Account: a.Team.ID, AccountName: a.Team.Name, App: a.AppID, Raw: a.raw,
		Access:      token(a.AccessToken, a.ExpiresIn, now),
		Refresh:     token(a.RefreshToken, 0, now),
		UserAccess:  token(a.AuthedUser.AccessToken, a.AuthedUser.ExpiresIn, now),
		UserRefresh: token(a.AuthedUser.RefreshToken, 0, now),
	}
}

/*
token returns the token expiring the seconds after now, nil when empty.
*/
func token(value string, expiresIn int64, now time.Time) *oauthclient.Token {
	if value == "" {
		return nil
	}

	t := &oauthclient.Token{Value: value}
	if expiresIn > 0 {
		t.Expiry = now.Add(time.Duration(expiresIn) * time.Second)
	}

	return t
}

/*
scopes splits Slack's comma-joined scope list.
*/
func scopes(list string) []string {
	if list == "" {
		return nil
	}

	return strings.Split(list, ",")
}
