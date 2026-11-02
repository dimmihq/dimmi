/*
|--------------------------------------------------------------------------
| Session Manager
|--------------------------------------------------------------------------
|
| The manager turns a cookie into a session and back. The cookie carries
| a random token and nothing else, readable by no script and sent only to
| this host over HTTPS: the __Host- prefix makes the browser refuse any
| cookie of that name set by a subdomain or without Secure.
|
| A session ends on its own after sitting idle too long and, however
| active, at its absolute lifetime. An ended session is treated as if the
| browser had none, so the visitor simply logs in again.
|
*/

package session

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/hashing"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
)

/*
hostPrefix is the cookie name prefix browsers only accept on a Secure,
host-only cookie for the whole site.
*/
const hostPrefix = "__Host-"

/*
tokenBytes is the number of random bytes in a session token.
*/
const tokenBytes = 32

/*
Config configures the manager.

Cookie is the cookie name, "session" by default; served over HTTPS it
gets the __Host- prefix. Insecure drops the prefix and the Secure flag,
for a plain-HTTP install on a local network only. SameSite is "lax"
(the default), "strict" or "none". IdleTimeout ends a session nobody used
for that long (default 14 days), Lifetime ends any session that long
after login (default 30 days), and TouchInterval is how often activity
is written to the store (default five minutes). Errors renders a store
failure as the response.
*/
type Config struct {
	Cookie        string        `koanf:"cookie"`
	Insecure      bool          `koanf:"insecure"`
	SameSite      string        `koanf:"same_site"`
	IdleTimeout   time.Duration `koanf:"idle_timeout"`
	Lifetime      time.Duration `koanf:"lifetime"`
	TouchInterval time.Duration `koanf:"touch_interval"`
	Errors        fhttp.Errors  `koanf:"-"`
}

/*
Manager loads and saves sessions.
*/
type Manager struct {
	store Store
	clock clock.Clock
	cfg   Config
}

/*
NewManager returns a manager on the store. Zero settings fall back to
their defaults.
*/
func NewManager(store Store, clk clock.Clock, cfg Config) *Manager {
	return &Manager{store: store, clock: clk, cfg: withDefaults(cfg)}
}

/*
Configure replaces every setting of the manager, zero ones falling back
to their defaults as in NewManager. An application whose own
configuration names the cookie and lifetimes calls it while it boots,
before any request is served, and keeps the framework's store.
*/
func (m *Manager) Configure(cfg Config) {
	m.cfg = withDefaults(cfg)
}

/*
CookieName returns the name of the session cookie, with its prefix.
*/
func (m *Manager) CookieName() string {
	if m.cfg.Insecure {
		return m.cfg.Cookie
	}

	return hostPrefix + m.cfg.Cookie
}

/*
Start returns the request's session: the stored one its cookie names
when that is still live, otherwise a new, empty one that is stored only
once something is put in it.
*/
func (m *Manager) Start(r *http.Request) (*Session, error) {
	now := m.clock.Now()

	token := cookieValue(r, m.CookieName())
	if token == "" {
		return m.fresh(r, now), nil
	}

	record, err := m.store.Find(r.Context(), digest(token))
	if errors.Is(err, ErrNotFound) {
		return m.fresh(r, now), nil
	}

	if err != nil {
		return nil, err
	}

	if m.ended(record, now) {
		if err := m.store.Delete(r.Context(), record.ID); err != nil {
			return nil, err
		}

		return m.fresh(r, now), nil
	}

	old := record.Flash
	record.Flash = map[string]string{}

	return &Session{
		record: record, old: old, now: now, lifetime: m.cfg.Lifetime, stored: true,
		dirty: len(old) > 0,
	}, nil
}

/*
Sessions returns the user's live sessions, most recently active first,
for a list of the devices they are signed in on.
*/
func (m *Manager) Sessions(ctx context.Context, userID ids.UUID) ([]Record, error) {
	records, err := m.store.ForUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	now := m.clock.Now()

	return slices.DeleteFunc(records, func(r Record) bool { return m.ended(r, now) }), nil
}

/*
Revoke signs the user out of one of their sessions. It returns
ErrNotFound when the session is not the user's, so nobody can end
another person's session by guessing its id.
*/
func (m *Manager) Revoke(ctx context.Context, userID, id ids.UUID) error {
	records, err := m.store.ForUser(ctx, userID)
	if err != nil {
		return err
	}

	if !slices.ContainsFunc(records, func(r Record) bool { return r.ID == id }) {
		return ErrNotFound
	}

	return m.store.Delete(ctx, id)
}

/*
RevokeOthers signs the user out everywhere except the kept session, and
returns how many sessions ended.
*/
func (m *Manager) RevokeOthers(ctx context.Context, userID, keep ids.UUID) (int64, error) {
	return m.store.DeleteForUser(ctx, userID, keep)
}

/*
commit writes the session back to the store and sets or clears the
cookie, before the response's headers go out.
*/
func (m *Manager) commit(ctx context.Context, w http.ResponseWriter, s *Session) error {
	if s.destroyed {
		return m.destroy(ctx, w, s)
	}

	if !s.stored && s.empty() {
		return nil
	}

	if !s.dirty && s.stored {
		return m.touch(ctx, s)
	}

	s.record.LastActivityAt, s.record.UpdatedAt = s.now, s.now
	if err := m.store.Save(ctx, s.record); err != nil {
		return err
	}

	if s.previous != ids.Nil() {
		if err := m.store.Delete(ctx, s.previous); err != nil {
			return fmt.Errorf("delete rotated session: %w", err)
		}
	}

	if s.token != "" {
		http.SetCookie(w, m.cookie(s.token, s.record.ExpiresAt.Sub(s.now)))
	}

	return nil
}

func (m *Manager) touch(ctx context.Context, s *Session) error {
	if s.now.Sub(s.record.LastActivityAt) < m.cfg.TouchInterval {
		return nil
	}

	return m.store.Touch(ctx, s.record.ID, s.now)
}

func (m *Manager) destroy(ctx context.Context, w http.ResponseWriter, s *Session) error {
	for _, id := range []ids.UUID{s.previous, s.record.ID} {
		if id == ids.Nil() {
			continue
		}

		if err := m.store.Delete(ctx, id); err != nil {
			return err
		}
	}

	http.SetCookie(w, m.cookie("", -time.Second))

	return nil
}

func (m *Manager) fresh(r *http.Request, now time.Time) *Session {
	token := newToken()

	return &Session{
		record: Record{
			ID:             ids.NewV7At(now),
			TokenHash:      digest(token),
			Data:           map[string]string{},
			Flash:          map[string]string{},
			IP:             fhttp.ClientIP(r),
			UserAgent:      r.UserAgent(),
			LastActivityAt: now,
			ExpiresAt:      now.Add(m.cfg.Lifetime),
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		old:      map[string]string{},
		now:      now,
		lifetime: m.cfg.Lifetime,
		token:    token,
	}
}

/*
ended reports whether the session reached its absolute lifetime or sat
idle for the whole idle timeout. Both limits are inclusive: a session
idle exactly the timeout has ended.
*/
func (m *Manager) ended(r Record, now time.Time) bool {
	return !now.Before(r.ExpiresAt) || now.Sub(r.LastActivityAt) >= m.cfg.IdleTimeout
}

/*
cookie returns the session cookie with the token, kept for maxAge. A
negative maxAge tells the browser to delete it.
*/
func (m *Manager) cookie(token string, maxAge time.Duration) *http.Cookie {
	c := &http.Cookie{
		Name:     m.CookieName(),
		Value:    token,
		Path:     "/",
		MaxAge:   int(maxAge.Seconds()),
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}

	c.Secure = !m.cfg.Insecure
	c.SameSite = sameSite(m.cfg.SameSite)

	return c
}

/*
cookieValue returns the value of the first cookie with the name, or ""
when the request carries none.
*/
func cookieValue(r *http.Request, name string) string {
	for _, c := range r.CookiesNamed(name) {
		return c.Value
	}

	return ""
}

func sameSite(mode string) http.SameSite {
	switch mode {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}

/*
newToken returns a new random cookie token. The system random source
never fails, so there is no error to return.
*/
func newToken() string {
	raw := make([]byte, tokenBytes)
	_, _ = rand.Read(raw)

	return base64.RawURLEncoding.EncodeToString(raw)
}

func digest(token string) []byte {
	return hashing.SHA256([]byte(token))
}

func withDefaults(cfg Config) Config {
	cfg.Cookie = orDefault(cfg.Cookie, "session")
	cfg.SameSite = orDefault(strings.ToLower(cfg.SameSite), "lax")
	cfg.IdleTimeout = orDefault(cfg.IdleTimeout, 14*24*time.Hour)
	cfg.Lifetime = orDefault(cfg.Lifetime, 30*24*time.Hour)
	cfg.TouchInterval = orDefault(cfg.TouchInterval, 5*time.Minute)

	return cfg
}

func orDefault[T comparable](value, fallback T) T {
	var zero T
	if value == zero {
		return fallback
	}

	return value
}
