/*
|--------------------------------------------------------------------------
| Signed-in Readers
|--------------------------------------------------------------------------
|
| A reader signs in to a help center through the team's own product:
| the help center sends them to its login address, the product signs an
| identity token for them, as it does for the messenger, and sends them
| back with it in the address's fragment, which never reaches a server
| log. The token is checked like the messenger's, works once when it
| carries a nonce, and names the customer by their user id. However they
| signed in, by token, email code or a pass from the messenger, the
| reader is then remembered for seven days by a signed cookie that only
| this help center reads, renewed when they come back. Signing in writes
| what the token says about the customer, as the messenger does, so the
| audiences that keep articles see the same customer either way.
|
*/

package helpcenters

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/ratelimit"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/identity"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ReaderLifetime is how long a signed-in reader is remembered after their
last visit; readerRenewAfter is how soon a visit renews it.
*/
const (
	ReaderLifetime   = 7 * 24 * time.Hour
	readerRenewAfter = 24 * time.Hour
)

/*
ReaderAudience is the audience a help center's identity token may name.
*/
const ReaderAudience = "dimmi-help"

/*
ReaderSession is a signed-in reader as the cookie keeps them: its Value
and when it ends.
*/
type ReaderSession struct {
	Value string
	Until time.Time
}

/*
Readers signs readers in to help centers and recognizes them after.
Cache keeps codes and passes, Limiter throttles codes, Mail sends them,
and Key signs the cookie that remembers readers.
*/
type Readers struct {
	DB       *database.DB
	Clock    clock.Clock
	Verifier *identity.Verifier
	Events   *events.Dispatcher
	Cache    *cache.Store
	Limiter  *ratelimit.Limiter
	Mail     *mail.Manager
	Key      []byte
}

/*
SignIn checks a reader's identity token and returns the session that
remembers them. It works inside the help center's
workspace, since a public page names no workspace of its own.
*/
func (r *Readers) SignIn(ctx context.Context, h models.HelpCenter, token string) (ReaderSession, error) {
	ctx = tenancy.With(ctx, h.WorkspaceID.String())
	verified, err := r.Verifier.Verify(ctx, identity.Token{
		WorkspaceID: h.WorkspaceID, AppID: ReaderAudience, Value: token,
	})
	if err != nil {
		return ReaderSession{}, err
	}

	now := r.Clock.Now()
	var contact ids.UUID
	err = r.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		found, err := identity.Identify(ctx, tx, r.Events, identity.Visitor{
			WorkspaceID: h.WorkspaceID, Profile: verified.Profile, Now: now,
		})
		contact = found.Contact.ID

		return err
	})
	if err != nil {
		return ReaderSession{}, fmt.Errorf("identify reader: %w", err)
	}

	return r.session(h, contact), nil
}

/*
Reader returns the contact a cookie value remembers for the help center
and when it ends, false when it is missing, forged, for another help
center or too old.
*/
func (r *Readers) Reader(h models.HelpCenter, value string) (ids.UUID, time.Time, bool) {
	payload, mac, ok := strings.Cut(value, ".")
	if !ok || !hashing.Equal([]byte(mac), []byte(r.mac(payload))) {
		return ids.Nil(), time.Time{}, false
	}

	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return ids.Nil(), time.Time{}, false
	}

	parts := strings.Split(string(raw), "|")
	if len(parts) != 3 || parts[0] != h.ID.String() {
		return ids.Nil(), time.Time{}, false
	}

	end, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || r.Clock.Now().Unix() >= end {
		return ids.Nil(), time.Time{}, false
	}

	contact, err := ids.Parse(parts[1])

	return contact, time.Unix(end, 0), err == nil
}

/*
Renewal returns a fresh session for a reader who came back, false while
the one they have is still new.
*/
func (r *Readers) Renewal(h models.HelpCenter, contact ids.UUID, until time.Time) (ReaderSession, bool) {
	if until.Sub(r.Clock.Now()) > ReaderLifetime-readerRenewAfter {
		return ReaderSession{}, false
	}

	return r.session(h, contact), true
}

/*
session signs a fresh session for the reader of the help center.
*/
func (r *Readers) session(h models.HelpCenter, contact ids.UUID) ReaderSession {
	until := r.Clock.Now().Add(ReaderLifetime)

	return ReaderSession{Value: r.sign(h.ID, contact, until), Until: until}
}

/*
sign makes a cookie value naming the help center, the contact and the
end of the session, with its signature.
*/
func (r *Readers) sign(center, contact ids.UUID, until time.Time) string {
	payload := base64.RawURLEncoding.EncodeToString(
		[]byte(center.String() + "|" + contact.String() + "|" + strconv.FormatInt(until.Unix(), 10)),
	)

	return payload + "." + r.mac(payload)
}

func (r *Readers) mac(payload string) string {
	return base64.RawURLEncoding.EncodeToString(hashing.HMAC(r.Key, []byte(payload)))
}
