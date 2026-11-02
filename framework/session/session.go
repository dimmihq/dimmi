/*
|--------------------------------------------------------------------------
| Session
|--------------------------------------------------------------------------
|
| A session is what the server remembers about one browser between
| requests: who logged in, when they last proved it, and small values a
| handler wants back on the next request. Handlers change it freely; the
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
Record returns a copy of the session as it will be stored.
*/
func (s *Session) Record() Record {
	r := s.record
	r.Data, r.Flash = maps.Clone(r.Data), maps.Clone(r.Flash)

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
fresh absolute lifetime.
*/
func (s *Session) Login(userID ids.UUID) {
	s.record.UserID = userID
	s.record.AuthenticatedAt = s.now
	s.record.ConfirmedAt = s.now
	s.record.ExpiresAt = s.now.Add(s.lifetime)
	s.Regenerate()
}

/*
Confirm records that the user just proved their identity again, such as
by entering their password before a sensitive change. It is not a new
login, so AuthenticatedAt stays as it was.
*/
func (s *Session) Confirm() {
	s.record.ConfirmedAt = s.now
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
