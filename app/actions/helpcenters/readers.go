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
| carries a nonce, and names the customer by their user id; the reader
| is then remembered for an hour by a signed cookie that only this help
| center reads. Signing in writes what the token says about the customer,
| as the messenger does, so the audiences that keep articles see the
| same customer either way.
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

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/identity"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ReaderLifetime is how long a signed-in reader is remembered.
*/
const ReaderLifetime = time.Hour

/*
ReaderAudience is the audience a help center's identity token may name.
*/
const ReaderAudience = "dimmi-help"

/*
Readers signs readers in to help centers and recognizes them after.
Key signs the cookie that remembers them.
*/
type Readers struct {
	DB       *database.DB
	Clock    clock.Clock
	Verifier *identity.Verifier
	Events   *events.Dispatcher
	Key      []byte
}

/*
SignIn checks a reader's identity token and returns the cookie value that
remembers them, and when it ends. It works inside the help center's
workspace, since a public page names no workspace of its own.
*/
func (r *Readers) SignIn(ctx context.Context, h models.HelpCenter, token string) (string, time.Time, error) {
	ctx = tenancy.With(ctx, h.WorkspaceID.String())
	verified, err := r.Verifier.Verify(ctx, identity.Token{
		WorkspaceID: h.WorkspaceID, AppID: ReaderAudience, Value: token,
	})
	if err != nil {
		return "", time.Time{}, err
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
		return "", time.Time{}, fmt.Errorf("identify reader: %w", err)
	}

	until := now.Add(ReaderLifetime)

	return r.sign(h.ID, contact, until), until, nil
}

/*
Reader returns the contact a cookie value remembers for the help center,
false when it is missing, forged, for another help center or too old.
*/
func (r *Readers) Reader(h models.HelpCenter, value string) (ids.UUID, bool) {
	payload, mac, ok := strings.Cut(value, ".")
	if !ok || !hashing.Equal([]byte(mac), []byte(r.mac(payload))) {
		return ids.Nil(), false
	}

	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return ids.Nil(), false
	}

	parts := strings.Split(string(raw), "|")
	if len(parts) != 3 || parts[0] != h.ID.String() {
		return ids.Nil(), false
	}

	until, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || r.Clock.Now().Unix() >= until {
		return ids.Nil(), false
	}

	contact, err := ids.Parse(parts[1])

	return contact, err == nil
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
