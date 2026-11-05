/*
|--------------------------------------------------------------------------
| Connected Accounts
|--------------------------------------------------------------------------
|
| A mailbox reached through its provider's own API, rather than through
| mail delivered to a webhook. Nothing is pushed reliably, so the account
| keeps a cursor: the application asks what changed since, handles each
| new message, and moves the cursor only after. A cursor that has gone
| stale is recovered by listing recent mail, which the application
| dedupes.
|
| The account is handed an access token on every call and never keeps
| one: the token lives sealed with the application, which refreshes it.
|
*/

package mailbox

import (
	"context"
	"errors"
	"fmt"
	"time"
)

/*
Errors an account returns.

ErrCursorExpired means the provider no longer knows the cursor.
ErrGrantRevoked means the sign-in grant ended: revoked, expired or its
password changed. ErrAdminBlocked means an administrator blocked the
application. ErrAccountGone means the mailbox is out of reach for now:
removed, unlicensed or no longer shared. ErrSendRefused means the
provider will never send this message.
*/
var (
	ErrCursorExpired = errors.New("mailbox: cursor expired")
	ErrGrantRevoked  = errors.New("mailbox: grant revoked")
	ErrAdminBlocked  = errors.New("mailbox: blocked by an administrator")
	ErrAccountGone   = errors.New("mailbox: account out of reach")
	ErrSendRefused   = errors.New("mailbox: send refused")
)

/*
RateLimitedError means the provider asked to slow down for RetryAfter,
which is zero when it did not say for how long.
*/
type RateLimitedError struct {
	RetryAfter time.Duration
}

/*
Error says how long to wait.
*/
func (e *RateLimitedError) Error() string {
	return fmt.Sprintf("mailbox: rate limited, retry after %s", e.RetryAfter)
}

/*
Folder says where a message of an account sits.
*/
type Folder string

/*
The folders read: mail that came in, and mail the account sent.
*/
const (
	FolderReceived Folder = "received"
	FolderSent     Folder = "sent"
)

/*
Session is one call's access to an account: the access token and the
mailbox to act on, which differs from the signed-in account for a shared
mailbox.
*/
type Session struct {
	Token   string
	Mailbox string
}

/*
Profile is who signed in. Aliases are the other addresses the provider
confirms the account may send as.
*/
type Profile struct {
	Email   string
	Aliases []string
}

/*
Change is one new message of an account.
*/
type Change struct {
	ID           string
	ThreadID     string
	Folder       Folder
	RFCMessageID string
}

/*
Changes is one page of what changed. Items list sent mail first, so a
reply to something just sent finds what it refers to. Cursor is what to
ask with next; More says another page waits right now.
*/
type Changes struct {
	Items  []Change
	Cursor []byte
	More   bool
}

/*
Account is a source read through a provider's API.
*/
type Account interface {
	Source

	/*
		Profile returns who signed in.
	*/
	Profile(ctx context.Context, s Session) (Profile, error)

	/*
		Cursor returns a cursor at this moment, so nothing older is read.
	*/
	Cursor(ctx context.Context, s Session) ([]byte, error)

	/*
		Changes returns what changed since the cursor, or
		ErrCursorExpired.
	*/
	Changes(ctx context.Context, s Session, cursor []byte) (Changes, error)

	/*
		Since returns recent mail after a cursor expired, and a new cursor.
	*/
	Since(ctx context.Context, s Session, since time.Time) (Changes, error)

	/*
		Raw returns the raw message.
	*/
	Raw(ctx context.Context, s Session, id string) ([]byte, error)

	/*
		Verdicts returns what the provider, which received the connection,
		concluded about a raw message.
	*/
	Verdicts(raw []byte) Verdicts

	/*
		Send sends a raw message from the mailbox.
	*/
	Send(ctx context.Context, s Session, in SendInput) (Sent, error)

	/*
		Watch starts or renews announcements of changes.
	*/
	Watch(ctx context.Context, s Session, in WatchInput) (Watching, error)

	/*
		StopWatch ends announcements.
	*/
	StopWatch(ctx context.Context, s Session, state []byte) error

	/*
		Revoke ends the grant at the provider, when the provider has a way.
	*/
	Revoke(ctx context.Context, refreshToken string) error
}
