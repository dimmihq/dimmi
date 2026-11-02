package session_test

import (
	stdhttp "net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database/migrate"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/session"
)

func TestMigrations(t *testing.T) {
	t.Parallel()

	t.Run("pass the migration lint", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, migrate.Lint(session.Migrations()))
	})
}

func TestCookie(t *testing.T) {
	t.Parallel()

	db := openDB(t)

	t.Run("is not set for a visitor with nothing stored", func(t *testing.T) {
		t.Parallel()
		a := newApp(t, db, session.Config{})

		rec := a.send(t, http.MethodGet, "/get?key=missing", nil)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Nil(t, a.sessionCookie(rec))
	})

	t.Run("is host-only, secure and hidden from scripts", func(t *testing.T) {
		t.Parallel()
		a := newApp(t, db, session.Config{})

		rec := a.send(t, http.MethodPost, "/put?key=color&value=blue", nil)
		cookie := a.sessionCookie(rec)

		require.NotNil(t, cookie)
		assert.Equal(t, "__Host-session", cookie.Name)
		assert.Equal(t, "/", cookie.Path)
		assert.Empty(t, cookie.Domain)
		assert.True(t, cookie.Secure)
		assert.True(t, cookie.HttpOnly)
		assert.Equal(t, stdhttp.SameSiteLaxMode, cookie.SameSite)
		assert.Equal(t, int((30 * 24 * time.Hour).Seconds()), cookie.MaxAge)
	})

	t.Run("drops the prefix and secure flag on plain http", func(t *testing.T) {
		t.Parallel()
		a := newApp(t, db, session.Config{Cookie: "sid", Insecure: true, SameSite: "Strict"})

		cookie := a.sessionCookie(a.send(t, http.MethodPost, "/put?key=color&value=blue", nil))

		require.NotNil(t, cookie)
		assert.Equal(t, "sid", cookie.Name)
		assert.False(t, cookie.Secure)
		assert.Equal(t, stdhttp.SameSiteStrictMode, cookie.SameSite)
	})

	t.Run("allows cross-site sending when configured", func(t *testing.T) {
		t.Parallel()
		a := newApp(t, db, session.Config{SameSite: "none"})

		cookie := a.sessionCookie(a.send(t, http.MethodPost, "/put?key=color&value=blue", nil))

		require.NotNil(t, cookie)
		assert.Equal(t, stdhttp.SameSiteNoneMode, cookie.SameSite)
	})

	t.Run("stores only a digest of the token", func(t *testing.T) {
		t.Parallel()
		a := newApp(t, db, session.Config{})
		cookie := a.sessionCookie(a.send(t, http.MethodPost, "/put?key=color&value=blue", nil))

		record, err := a.store.Find(t.Context(), hashing.SHA256([]byte(cookie.Value)))

		require.NoError(t, err)
		assert.Equal(t, "blue", record.Data["color"])
		assert.Equal(t, "192.0.2.1", record.IP)
		assert.Equal(t, "test-agent", record.UserAgent)
		assert.NotContains(t, string(record.TokenHash), cookie.Value)
	})
}

func TestSessionData(t *testing.T) {
	t.Parallel()

	db := openDB(t)

	t.Run("comes back on the next request", func(t *testing.T) {
		t.Parallel()
		a := newApp(t, db, session.Config{})
		cookie := a.sessionCookie(a.send(t, http.MethodPost, "/put?key=color&value=blue", nil))

		rec := a.send(t, http.MethodGet, "/get?key=color", cookie)

		assert.Equal(t, "blue", rec.Body.String())
		assert.Nil(t, a.sessionCookie(rec))
	})

	t.Run("can be forgotten", func(t *testing.T) {
		t.Parallel()
		a := newApp(t, db, session.Config{})
		cookie := a.sessionCookie(a.send(t, http.MethodPost, "/put?key=color&value=blue", nil))

		a.send(t, http.MethodPost, "/forget?key=missing", cookie)
		a.send(t, http.MethodPost, "/forget?key=color", cookie)

		assert.Empty(t, a.send(t, http.MethodGet, "/get?key=color", cookie).Body.String())
	})

	t.Run("survives a handler that writes nothing", func(t *testing.T) {
		t.Parallel()
		a := newApp(t, db, session.Config{})

		cookie := a.sessionCookie(a.send(t, http.MethodPost, "/flash", nil))

		require.NotNil(t, cookie)
		assert.Equal(t, "saved", a.send(t, http.MethodGet, "/flashed", cookie).Body.String())
	})

	t.Run("flashes for the next request only", func(t *testing.T) {
		t.Parallel()
		a := newApp(t, db, session.Config{})
		cookie := a.sessionCookie(a.send(t, http.MethodPost, "/put?key=color&value=blue", nil))

		a.send(t, http.MethodPost, "/flash", cookie)

		assert.Equal(t, "saved", a.send(t, http.MethodGet, "/flashed", cookie).Body.String())
		assert.Empty(t, a.send(t, http.MethodGet, "/flashed", cookie).Body.String())
	})

	t.Run("starts afresh for an unknown token", func(t *testing.T) {
		t.Parallel()
		a := newApp(t, db, session.Config{})

		rec := a.send(t, http.MethodGet, "/get?key=color", &http.Cookie{Name: "__Host-session", Value: "forged"})

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Empty(t, rec.Body.String())
	})

	t.Run("is copied out by record", func(t *testing.T) {
		t.Parallel()
		a := newApp(t, db, session.Config{})
		req := newRequest(t)

		s, err := a.manager.Start(req)
		require.NoError(t, err)
		s.Put("color", "blue")
		record := s.Record()
		record.Data["color"] = "red"

		value, _ := s.Get("color")
		assert.Equal(t, "blue", value)
		_, ok := s.UserID()
		assert.False(t, ok)
		assert.True(t, s.AuthenticatedAt().IsZero())
	})
}

func TestLogin(t *testing.T) {
	t.Parallel()

	db := openDB(t)

	t.Run("binds the user and records the login time", func(t *testing.T) {
		t.Parallel()
		a := newApp(t, db, session.Config{})
		user := ids.New()
		cookie := a.login(t, user)

		record, err := a.store.Find(t.Context(), hashing.SHA256([]byte(cookie.Value)))

		require.NoError(t, err)
		assert.Equal(t, user, record.UserID)
		assert.Equal(t, a.clock.Now(), record.AuthenticatedAt.UTC())
		assert.Equal(t, a.clock.Now(), record.ConfirmedAt.UTC())
	})

	t.Run("rotates the id and token so a planted cookie dies", func(t *testing.T) {
		t.Parallel()
		a := newApp(t, db, session.Config{})
		planted := a.sessionCookie(a.send(t, http.MethodPost, "/put?key=color&value=blue", nil))
		before, err := a.store.Find(t.Context(), hashing.SHA256([]byte(planted.Value)))
		require.NoError(t, err)

		rec := a.send(t, http.MethodPost, "/login?user="+ids.New().String(), planted)
		fresh := a.sessionCookie(rec)

		require.NotNil(t, fresh)
		assert.NotEqual(t, planted.Value, fresh.Value)
		assert.NotEqual(t, before.ID.String(), rec.Body.String())
		_, err = a.store.Find(t.Context(), hashing.SHA256([]byte(planted.Value)))
		require.ErrorIs(t, err, session.ErrNotFound)
		assert.Equal(t, "blue", a.send(t, http.MethodGet, "/get?key=color", fresh).Body.String())
		assert.Equal(t, http.StatusUnauthorized, a.send(t, http.MethodGet, "/me", planted).Code)
	})

	t.Run("regenerates on demand without leaving rows", func(t *testing.T) {
		t.Parallel()
		a := newApp(t, db, session.Config{})
		user := ids.New()
		cookie := a.login(t, user)

		rotated := a.sessionCookie(a.send(t, http.MethodPost, "/regenerate", cookie))
		require.NotNil(t, rotated)

		records, err := a.manager.Sessions(t.Context(), user)
		require.NoError(t, err)
		assert.Len(t, records, 1)
		assert.Equal(t, http.StatusOK, a.send(t, http.MethodGet, "/me", rotated).Code)
	})

	t.Run("confirms without counting as a login", func(t *testing.T) {
		t.Parallel()
		a := newApp(t, db, session.Config{})
		cookie := a.login(t, ids.New())
		loggedIn := a.clock.Now()

		a.clock.Travel(time.Hour)
		a.send(t, http.MethodPost, "/confirm?method=password", cookie)

		record, err := a.store.Find(t.Context(), hashing.SHA256([]byte(cookie.Value)))
		require.NoError(t, err)
		assert.Equal(t, loggedIn, record.AuthenticatedAt.UTC())
		assert.Equal(t, a.clock.Now(), record.ConfirmedAt.UTC())
	})

	t.Run("ends at logout and clears the cookie", func(t *testing.T) {
		t.Parallel()
		a := newApp(t, db, session.Config{})
		cookie := a.login(t, ids.New())

		cleared := a.sessionCookie(a.send(t, http.MethodPost, "/logout", cookie))

		require.NotNil(t, cleared)
		assert.Empty(t, cleared.Value)
		assert.Negative(t, cleared.MaxAge)
		assert.Equal(t, http.StatusUnauthorized, a.send(t, http.MethodGet, "/me", cookie).Code)
	})
}

func TestExpiry(t *testing.T) {
	t.Parallel()

	db := openDB(t)

	t.Run("ends a session left idle too long", func(t *testing.T) {
		t.Parallel()
		a := newApp(t, db, session.Config{IdleTimeout: time.Hour})
		cookie := a.login(t, ids.New())

		a.clock.Travel(time.Hour)

		assert.Equal(t, http.StatusUnauthorized, a.send(t, http.MethodGet, "/me", cookie).Code)
		_, err := a.store.Find(t.Context(), hashing.SHA256([]byte(cookie.Value)))
		require.ErrorIs(t, err, session.ErrNotFound)
	})

	t.Run("keeps an active session alive past the idle timeout", func(t *testing.T) {
		t.Parallel()
		a := newApp(t, db, session.Config{IdleTimeout: time.Hour, TouchInterval: time.Minute})
		cookie := a.login(t, ids.New())

		for range 3 {
			a.clock.Travel(50 * time.Minute)
			require.Equal(t, http.StatusOK, a.send(t, http.MethodGet, "/me", cookie).Code)
		}
	})

	t.Run("ends any session at its absolute lifetime", func(t *testing.T) {
		t.Parallel()
		a := newApp(t, db, session.Config{IdleTimeout: time.Hour, Lifetime: 2 * time.Hour})
		cookie := a.login(t, ids.New())

		a.clock.Travel(50 * time.Minute)
		require.Equal(t, http.StatusOK, a.send(t, http.MethodGet, "/me", cookie).Code)
		a.clock.Travel(50 * time.Minute)
		require.Equal(t, http.StatusOK, a.send(t, http.MethodGet, "/me", cookie).Code)
		a.clock.Travel(20 * time.Minute)

		assert.Equal(t, http.StatusUnauthorized, a.send(t, http.MethodGet, "/me", cookie).Code)
	})

	t.Run("writes activity at most once per interval", func(t *testing.T) {
		t.Parallel()
		a := newApp(t, db, session.Config{TouchInterval: 5 * time.Minute})
		cookie := a.login(t, ids.New())
		loggedIn := a.clock.Now()

		a.clock.Travel(time.Minute)
		a.send(t, http.MethodGet, "/me", cookie)
		record, err := a.store.Find(t.Context(), hashing.SHA256([]byte(cookie.Value)))
		require.NoError(t, err)
		assert.Equal(t, loggedIn, record.LastActivityAt.UTC())

		a.clock.Travel(5 * time.Minute)
		a.send(t, http.MethodGet, "/me", cookie)
		record, err = a.store.Find(t.Context(), hashing.SHA256([]byte(cookie.Value)))
		require.NoError(t, err)
		assert.Equal(t, a.clock.Now(), record.LastActivityAt.UTC())
	})

	t.Run("prunes ended sessions", func(t *testing.T) {
		t.Parallel()
		a := newApp(t, openDB(t), session.Config{IdleTimeout: time.Hour})
		stale := a.login(t, ids.New())
		a.clock.Travel(2 * time.Hour)
		live := a.login(t, ids.New())

		pruned, err := a.store.Prune(t.Context(), a.clock.Now(), a.clock.Now().Add(-time.Hour))

		require.NoError(t, err)
		assert.Equal(t, int64(1), pruned)
		_, err = a.store.Find(t.Context(), hashing.SHA256([]byte(stale.Value)))
		require.ErrorIs(t, err, session.ErrNotFound)
		_, err = a.store.Find(t.Context(), hashing.SHA256([]byte(live.Value)))
		require.NoError(t, err)
	})
}

func TestDevices(t *testing.T) {
	t.Parallel()

	db := openDB(t)

	t.Run("lists the user's live sessions, newest activity first", func(t *testing.T) {
		t.Parallel()
		a := newApp(t, db, session.Config{IdleTimeout: 3 * time.Hour})
		user := ids.New()
		a.login(t, user)
		a.clock.Travel(2 * time.Hour)
		a.login(t, user)
		newest := a.login(t, user)
		a.clock.Travel(2 * time.Hour)
		a.send(t, http.MethodGet, "/me", newest)

		records, err := a.manager.Sessions(t.Context(), user)

		require.NoError(t, err)
		require.Len(t, records, 2)
		assert.Equal(t, a.clock.Now(), records[0].LastActivityAt.UTC())
	})

	t.Run("revokes one of the user's own sessions", func(t *testing.T) {
		t.Parallel()
		a := newApp(t, db, session.Config{})
		user := ids.New()
		cookie := a.login(t, user)
		records, err := a.manager.Sessions(t.Context(), user)
		require.NoError(t, err)

		require.NoError(t, a.manager.Revoke(t.Context(), user, records[0].ID))

		assert.Equal(t, http.StatusUnauthorized, a.send(t, http.MethodGet, "/me", cookie).Code)
	})

	t.Run("refuses to revoke another user's session", func(t *testing.T) {
		t.Parallel()
		a := newApp(t, db, session.Config{})
		owner := ids.New()
		cookie := a.login(t, owner)
		records, err := a.manager.Sessions(t.Context(), owner)
		require.NoError(t, err)

		err = a.manager.Revoke(t.Context(), ids.New(), records[0].ID)

		require.ErrorIs(t, err, session.ErrNotFound)
		assert.Equal(t, http.StatusOK, a.send(t, http.MethodGet, "/me", cookie).Code)
	})

	t.Run("revokes every other session", func(t *testing.T) {
		t.Parallel()
		a := newApp(t, db, session.Config{})
		user := ids.New()
		other := a.login(t, user)
		current := a.login(t, user)
		_, currentID, _ := strings.Cut(a.send(t, http.MethodGet, "/me", current).Body.String(), " ")
		keep, err := ids.Parse(currentID)
		require.NoError(t, err)

		revoked, err := a.manager.RevokeOthers(t.Context(), user, keep)

		require.NoError(t, err)
		assert.Equal(t, int64(1), revoked)
		assert.Equal(t, http.StatusUnauthorized, a.send(t, http.MethodGet, "/me", other).Code)
		assert.Equal(t, http.StatusOK, a.send(t, http.MethodGet, "/me", current).Code)
	})
}

func TestStoreFailures(t *testing.T) {
	t.Parallel()

	t.Run("answer 500 when the session cannot be loaded", func(t *testing.T) {
		t.Parallel()
		db := openDB(t)
		a := newApp(t, db, session.Config{})
		cookie := a.login(t, ids.New())
		db.Close()

		rec := a.send(t, http.MethodGet, "/me", cookie)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.Contains(t, rec.Body.String(), "internal_error")
	})

	t.Run("answer 500 instead of the response when saving fails", func(t *testing.T) {
		t.Parallel()
		db := openDB(t)
		a := newApp(t, db, session.Config{})
		db.Close()

		rec := a.send(t, http.MethodPost, "/put?key=color&value=blue", nil)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.Nil(t, a.sessionCookie(rec))
	})

	t.Run("surface every store error", func(t *testing.T) {
		t.Parallel()
		db := openDB(t)
		a := newApp(t, db, session.Config{})
		db.Close()
		ctx, id := t.Context(), ids.New()

		require.Error(t, a.store.Touch(ctx, id, a.clock.Now()))
		require.Error(t, a.store.Delete(ctx, id))
		_, err := a.store.DeleteForUser(ctx, id, id)
		require.Error(t, err)
		_, err = a.store.Prune(ctx, a.clock.Now(), a.clock.Now())
		require.Error(t, err)
		_, err = a.manager.Sessions(ctx, id)
		require.Error(t, err)
		require.Error(t, a.manager.Revoke(ctx, id, id))
	})
}
