/*
|--------------------------------------------------------------------------
| Session
|--------------------------------------------------------------------------
|
| A session is what the server remembers about one browser between
| requests: who logged in, how and when they last proved it, the device
| it runs on, and small values a handler wants back on the next request. Handlers change it freely; the
| manager writes it back once, just before the response goes out, so a
| request that changes nothing writes nothing.
|
| Logging in always gives the browser a new session id and token. A token
| planted in the browser before login is worthless afterwards, which is
| what stops session fixation.
|
*/

package session

import (
	"context"
	"maps"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"
)

type contextKey struct{}

/*
Session is the session of the current request.
*/
type Session struct {
	record    Record
	old       map[string]string
	now       time.Time
	lifetime  time.Duration
	token     string
	previous  ids.UUID
	stored    bool
	dirty     bool
	destroyed bool
}

/*
From returns the session the middleware loaded for the request in the
context.
*/
func From(ctx context.Context) (*Session, bool) {
	s, ok := ctx.Value(contextKey{}).(*Session)

	return s, ok
}

/*
WithSession returns a copy of the context carrying the session.
*/
func WithSession(ctx context.Context, s *Session) context.Context {
	return context.WithValue(ctx, contextKey{}, s)
}

/*
ID returns the session's public id. It changes when the session is
regenerated.
*/
func (s *Session) ID() ids.UUID {
	return s.record.ID
}

/*
UserID returns the logged-in user, and false for a visitor.
*/
func (s *Session) UserID() (ids.UUID, bool) {
	return s.record.UserID, s.record.UserID != ids.Nil()
}

/*
AuthenticatedAt returns the time of the last full login, or the zero
time for a visitor.
*/
func (s *Session) AuthenticatedAt() time.Time {
	return s.record.AuthenticatedAt
}

/*
ConfirmedAt returns the last time the user proved their identity, by
logging in or confirming their password.
*/
func (s *Session) ConfirmedAt() time.Time {
	return s.record.ConfirmedAt
}

/*
AuthMethod returns the primary method of the last full login, such as
password, or "" for a visitor.
*/
func (s *Session) AuthMethod() string {
	return s.record.AuthMethod
}

/*
AuthMethods returns every method the last full login passed, the
primary one first.
*/
func (s *Session) AuthMethods() []string {
	return slices.Clone(s.record.AuthMethods)
}

/*
ConfirmedMethod returns the method the user last proved their identity
with, which a login sets to the last method it passed.
*/
func (s *Session) ConfirmedMethod() string {
	return s.record.ConfirmedMethod
}

/*
DeviceID returns the id of the device the session runs on, or "" when
it was never set.
*/
func (s *Session) DeviceID() string {
	return s.record.DeviceID
}

/*
DeviceLabel returns the name of the device shown to people, such as
"Firefox on macOS".
*/
func (s *Session) DeviceLabel() string {
	return s.record.DeviceLabel
}

/*
Country returns the two-letter country code the session started from,
or "" when unknown.
*/
func (s *Session) Country() string {
	return s.record.Country
}

/*
SetDevice records the device the session runs on: an id shared by every
session of that device and a label to show the user.
*/
func (s *Session) SetDevice(id, label string) {
	s.record.DeviceID, s.record.DeviceLabel = id, label
	s.dirty = true
}

/*
SetCountry records the two-letter ISO 3166 code of the country the
session started from.
*/
func (s *Session) SetCountry(country string) {
	s.record.Country = country
	s.dirty = true
}

/*
SetSignInContext records where the sign-in came from when an outside
service vouched for it: the connection it came through and the domain
that service stands for. Login clears both, so a later sign-in by
another way never keeps them.
*/
func (s *Session) SetSignInContext(connectionID ids.UUID, domain string) {
	s.record.ConnectionID, s.record.AuthDomain = connectionID, domain
	s.dirty = true
}

/*
Record returns a copy of the session as it will be stored.
*/
func (s *Session) Record() Record {
	r := s.record
	r.Data, r.Flash = maps.Clone(r.Data), maps.Clone(r.Flash)
	r.AuthMethods = slices.Clone(r.AuthMethods)

	return r
}

/*
Get returns the value stored under the key.
*/
func (s *Session) Get(key string) (string, bool) {
	value, ok := s.record.Data[key]

	return value, ok
}

/*
Put stores the value under the key.
*/
func (s *Session) Put(key, value string) {
	s.record.Data[key] = value
	s.dirty = true
}

/*
Forget removes the key.
*/
func (s *Session) Forget(key string) {
	if _, ok := s.record.Data[key]; !ok {
		return
	}

	delete(s.record.Data, key)
	s.dirty = true
}

/*
Flash stores the value for the next request only, such as a notice to
show after a redirect.
*/
func (s *Session) Flash(key, value string) {
	s.record.Flash[key] = value
	s.dirty = true
}

/*
Flashed returns the value the previous request flashed under the key.
*/
func (s *Session) Flashed(key string) (string, bool) {
	value, ok := s.old[key]

	return value, ok
}

/*
Login ties the session to the user, records the time as both the full
login and the last confirmation, and regenerates the session with a
fresh absolute lifetime. Methods are the ways the user proved who they
are, the primary one first: the first becomes AuthMethod and the last
ConfirmedMethod. A login may name none, and records no method then.
*/
func (s *Session) Login(userID ids.UUID, methods ...string) {
	s.record.UserID = userID
	s.record.AuthMethods = slices.Clone(methods)
	s.record.AuthMethod, s.record.ConfirmedMethod = "", ""

	if len(methods) > 0 {
		s.record.AuthMethod, s.record.ConfirmedMethod = methods[0], methods[len(methods)-1]
	}

	s.record.ConnectionID, s.record.AuthDomain = ids.Nil(), ""
	s.record.AuthenticatedAt = s.now
	s.record.ConfirmedAt = s.now
	s.record.ExpiresAt = s.now.Add(s.lifetime)
	s.Regenerate()
}

/*
Confirm records that the user just proved their identity again with the
method, such as by entering their password before a sensitive change.
It is not a new login, so AuthenticatedAt and AuthMethod stay as they
were.
*/
func (s *Session) Confirm(method string) {
	s.record.ConfirmedAt = s.now
	s.record.ConfirmedMethod = method
	s.dirty = true
}

/*
Regenerate gives the session a new id and token, keeping its contents.
The old session is deleted when the new one is saved. Call it whenever
the user's privileges change.
*/
func (s *Session) Regenerate() {
	if s.stored && s.previous == ids.Nil() {
		s.previous = s.record.ID
	}

	s.token = newToken()
	s.record.ID = ids.NewV7At(s.now)
	s.record.TokenHash = digest(s.token)
	s.record.CreatedAt = s.now
	s.dirty = true
}

/*
Destroy ends the session: it is deleted and the browser told to drop
the cookie.
*/
func (s *Session) Destroy() {
	s.destroyed = true
}

/*
empty reports whether the session holds nothing worth storing.
*/
func (s *Session) empty() bool {
	return s.record.UserID == ids.Nil() && len(s.record.Data) == 0 && len(s.record.Flash) == 0
}
