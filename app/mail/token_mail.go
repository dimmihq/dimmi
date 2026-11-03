/*
|--------------------------------------------------------------------------
| Token Mail
|--------------------------------------------------------------------------
|
| A personal access token is a standing key to a workspace, so its owner
| hears about it twice: when it is made, with what it may do and where it
| was made from, so a token they did not make never goes unnoticed; and
| a week before it expires, so a script that depends on it does not stop
| without warning.
|
*/

package mail

import (
	"context"
	"strings"

	"github.com/dimmihq/dimmi/framework/mail"

	"github.com/dimmihq/dimmi/resources/views/mail/accounts"
)

/*
TokenCreatedMail tells a user a token named Token was made in Workspace
with the Abilities, from the address IP, and links their tokens.
*/
type TokenCreatedMail struct {
	Name      string   `json:"name"`
	Token     string   `json:"token"`
	Workspace string   `json:"workspace"`
	Abilities []string `json:"abilities"`
	IP        string   `json:"ip"`
	TokensURL string   `json:"tokens_url"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (TokenCreatedMail) Envelope() mail.Envelope {
	return mail.Envelope{}
}

/*
Content renders the mail.
*/
func (m TokenCreatedMail) Content(ctx context.Context) (mail.Content, error) {
	abilities := strings.Join(m.Abilities, ", ")
	component := accounts.TokenCreated(m.Name, m.Token, m.Workspace, abilities, m.IP, m.TokensURL)

	return content(ctx, "mail.token_created.subject", component), nil
}

/*
TokenExpiringMail tells a user their token named Token in Workspace
stops working on ExpiresOn, a date already written for them, and links
their tokens.
*/
type TokenExpiringMail struct {
	Name      string `json:"name"`
	Token     string `json:"token"`
	Workspace string `json:"workspace"`
	ExpiresOn string `json:"expires_on"`
	TokensURL string `json:"tokens_url"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (TokenExpiringMail) Envelope() mail.Envelope {
	return mail.Envelope{}
}

/*
Content renders the mail.
*/
func (m TokenExpiringMail) Content(ctx context.Context) (mail.Content, error) {
	component := accounts.TokenExpiring(m.Name, m.Token, m.Workspace, m.ExpiresOn, m.TokensURL)

	return content(ctx, "mail.token_expiring.subject", component), nil
}
