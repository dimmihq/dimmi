/*
|--------------------------------------------------------------------------
| Session Store
|--------------------------------------------------------------------------
|
| The browser holds only a random token; everything else lives on the
| server. The store keys each session by a SHA-256 digest of that token,
| so a copy of the table never yields a cookie that logs anyone in. Each
| session also has an id of its own, which is safe to show, so a person
| can list the devices they are signed in on and sign one of them out.
|
*/

package session

import (
	"context"
	"errors"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
ErrNotFound means no session matches the token digest or id.
*/
var ErrNotFound = errors.New("session: not found")

/*
Record is one stored session.

ID is the public id and TokenHash the SHA-256 digest of the cookie
token. UserID is ids.Nil() for a visitor who has not logged in. Data holds
the values handlers put in the session and Flash the values flashed for
the next request. IP and UserAgent describe the device that started the
session.

The sign-in record says how the user got in. AuthMethod is the primary
method of the last full login and AuthMethods every method it passed,
the primary one first, such as password then totp. ConfirmedMethod is
how the user last proved their identity. DeviceID stays the same across
every session of one device, DeviceLabel names that device for people,
and Country is the two-letter ISO 3166 code the session started from.
Each is empty when unknown, and always for a visitor.

AuthenticatedAt is the last full login, ConfirmedAt the last time the
user proved their identity, by logging in or confirming their password.
LastActivityAt is written at most once per touch interval, UpdatedAt
whenever the session itself changes, and ExpiresAt is the absolute end
of the session.
*/
type Record struct {
	ID              ids.UUID
	TokenHash       []byte
	UserID          ids.UUID
	Data            map[string]string
	Flash           map[string]string
	IP              string
	UserAgent       string
	AuthMethod      string
	AuthMethods     []string
	ConfirmedMethod string
	DeviceID        string
	DeviceLabel     string
	Country         string
	AuthenticatedAt time.Time
	ConfirmedAt     time.Time
	LastActivityAt  time.Time
	ExpiresAt       time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

/*
Store keeps sessions on the server.
*/
type Store interface {
	/*
		Find returns the session whose token has the digest, or ErrNotFound.
	*/
	Find(ctx context.Context, tokenHash []byte) (Record, error)

	/*
		Save inserts the session, or replaces the stored one with the same id.
	*/
	Save(ctx context.Context, record Record) error

	/*
		Touch records activity on the session at the time.
	*/
	Touch(ctx context.Context, id ids.UUID, at time.Time) error

	/*
		Delete removes the session. Deleting a missing session is not an
		error.
	*/
	Delete(ctx context.Context, id ids.UUID) error

	/*
		ForUser returns the user's sessions, most recently active first.
	*/
	ForUser(ctx context.Context, userID ids.UUID) ([]Record, error)

	/*
		DeleteForUser removes every session of the user except the one
		kept, and returns how many it removed. Keeping ids.Nil() removes
		them all.
	*/
	DeleteForUser(ctx context.Context, userID, keep ids.UUID) (int64, error)
}
