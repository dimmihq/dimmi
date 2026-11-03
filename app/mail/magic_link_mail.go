/*
|--------------------------------------------------------------------------
| Magic Link Mail
|--------------------------------------------------------------------------
|
| Signs a person in without a password, by link on the device that reads
| the mail or by code on the one that asked, and says where the request
| came from so they can tell whether it was them.
|
*/

package mail

import (
	"context"

	"github.com/dimmihq/dimmi/framework/mail"

	"github.com/dimmihq/dimmi/resources/views/mail/accounts"
)

/*
MagicLinkMail signs a person in by URL or by Code within Minutes, and
names the IP and Device the request came from.
*/
type MagicLinkMail struct {
	URL     string `json:"url"`
	Code    string `json:"code"`
	IP      string `json:"ip"`
	Device  string `json:"device"`
	Minutes int    `json:"minutes"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (MagicLinkMail) Envelope() mail.Envelope {
	return mail.Envelope{}
}

/*
Content renders the mail.
*/
func (m MagicLinkMail) Content(ctx context.Context) (mail.Content, error) {
	view := accounts.MagicLink(m.URL, m.Code, m.IP, deviceName(ctx, m.Device), m.Minutes)

	return content(ctx, "mail.magic_link.subject", view), nil
}
