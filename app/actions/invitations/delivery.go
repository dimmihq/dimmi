/*
|--------------------------------------------------------------------------
| Invitation Delivery
|--------------------------------------------------------------------------
|
| Sending an invitation, the first time or again, always works the same
| way: a fresh link replaces any earlier one, so a link forwarded by
| mistake stops working the moment a new one goes out, and the link
| works for the invitation period from that moment. Only a digest of the
| link's secret is stored. The mail is queued in the transaction that
| issued the link, so a link is never valid without its mail on the way.
|
| A server without a mail transport writes mail to its log instead, and
| hands the link back to the admin who sent the invitation, to pass on
| by hand.
|
*/

package invitations

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	fmail "github.com/dimmihq/dimmi/framework/mail"

	appauth "github.com/dimmihq/dimmi/app/auth"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxEmailLength is the longest address an invitation goes to.
*/
const maxEmailLength = 254

/*
Sent is an invitation as it went out: the row, the name of who sent it,
and the link to accept it, so whoever sent it can also pass it on by
hand.
*/
type Sent struct {
	Invitation  models.Invitation
	InviterName string
	AcceptURL   string
}

/*
Delivery issues invitation links and mails them. TTL is how long a link
works.
*/
type Delivery struct {
	Clock clock.Clock
	Mail  *fmail.Manager
	Links appauth.Links
	TTL   time.Duration
}

/*
link is a fresh invitation link: its secret, the digest stored for it,
and until when it works.
*/
type link struct {
	token   string
	digest  []byte
	expires time.Time
}

/*
issue returns a fresh link.
*/
func (d Delivery) issue() link {
	token, digest := appauth.NewSecret()

	return link{token: token, digest: digest, expires: d.Clock.Now().Add(d.TTL)}
}

/*
outgoing is an invitation about to go out: its workspace, the row as
stored with the link, the name of who sends it, the link, and whether
the link is only handed back to pass on by hand, with no mail.
*/
type outgoing struct {
	workspace  models.Workspace
	invitation models.Invitation
	inviter    string
	link       link
	linkOnly   bool
}

/*
send queues the invitation mail with the link, in the transaction that
issued it, unless the link is only handed back, and returns the
invitation as it went out.
*/
func (d Delivery) send(ctx context.Context, tx *database.Tx, out outgoing) (Sent, error) {
	url := d.Links.Invitation(out.link.token)
	inv := out.invitation
	sent := Sent{Invitation: inv, InviterName: out.inviter, AcceptURL: url}

	if out.linkOnly {
		return sent, nil
	}

	pending := d.Mail.To(fmail.Address{Email: inv.Email}).Locale(out.workspace.Locale)

	err := pending.QueueTx(ctx, tx, appmail.InvitationMail{
		Workspace: out.workspace.Name, Inviter: out.inviter, Role: inv.Role, ExpiresAt: inv.ExpiresAt, AcceptURL: url,
	})
	if err != nil {
		return Sent{}, fmt.Errorf("mail invitation: %w", err)
	}

	return sent, nil
}

/*
normalizeEmail returns the address without outer spaces and with its
domain in lower case, and reports whether it is a bare email address.
*/
func normalizeEmail(raw string) (string, bool) {
	email := strings.TrimSpace(raw)
	if email == "" || len(email) > maxEmailLength {
		return email, false
	}

	parsed, err := fmail.ParseAddress(email)
	if err != nil || parsed.Name != "" || parsed.Email != email {
		return email, false
	}

	at := strings.LastIndex(email, "@")

	return email[:at] + strings.ToLower(email[at:]), true
}
