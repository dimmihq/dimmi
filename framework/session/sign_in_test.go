package session_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/session"
)

func TestSignInRecord(t *testing.T) {
	t.Parallel()

	db := openDB(t)

	t.Run("keeps how the user signed in", func(t *testing.T) {
		t.Parallel()
		a := newApp(t, db, session.Config{})
		user := ids.New()
		rec := a.send(t, http.MethodPost, "/login?methods=password,totp&user="+user.String(), nil)
		cookie := a.sessionCookie(rec)
		require.NotNil(t, cookie)

		record, err := a.store.Find(t.Context(), hashing.SHA256([]byte(cookie.Value)))

		require.NoError(t, err)
		assert.Equal(t, "password", record.AuthMethod)
		assert.Equal(t, []string{"password", "totp"}, record.AuthMethods)
		assert.Equal(t, "totp", record.ConfirmedMethod)
		assert.Equal(t, a.clock.Now(), record.UpdatedAt.UTC())
		assert.Equal(t, "password|password+totp|totp|||", a.send(t, http.MethodGet, "/sign-in", cookie).Body.String())
	})

	t.Run("keeps the device and country the session runs from", func(t *testing.T) {
		t.Parallel()
		a := newApp(t, db, session.Config{})
		cookie := a.login(t, ids.New())
		a.clock.Travel(time.Minute)

		a.send(t, http.MethodPost, "/device?id=laptop&label=Firefox+on+macOS&country=NL", cookie)

		record, err := a.store.Find(t.Context(), hashing.SHA256([]byte(cookie.Value)))
		require.NoError(t, err)
		assert.Equal(t, "laptop", record.DeviceID)
		assert.Equal(t, "Firefox on macOS", record.DeviceLabel)
		assert.Equal(t, "NL", record.Country)
		assert.Equal(t, a.clock.Now(), record.UpdatedAt.UTC())
		assert.Equal(t, "|||laptop|Firefox on macOS|NL", a.send(t, http.MethodGet, "/sign-in", cookie).Body.String())
	})

	t.Run("replaces the methods at the next login on the same browser", func(t *testing.T) {
		t.Parallel()
		a := newApp(t, db, session.Config{})
		user := ids.New()
		first := a.sessionCookie(a.send(t, http.MethodPost, "/login?methods=password,totp&user="+user.String(), nil))
		a.send(t, http.MethodPost, "/device?id=laptop&label=Laptop&country=NL", first)

		again := a.sessionCookie(a.send(t, http.MethodPost, "/login?methods=passkey&user="+user.String(), first))

		require.NotNil(t, again)
		signIn := a.send(t, http.MethodGet, "/sign-in", again).Body.String()
		assert.Equal(t, "passkey|passkey|passkey|laptop|Laptop|NL", signIn)
	})

	t.Run("confirms with a new method and keeps the login method", func(t *testing.T) {
		t.Parallel()
		a := newApp(t, db, session.Config{})
		cookie := a.sessionCookie(a.send(t, http.MethodPost, "/login?methods=passkey&user="+ids.New().String(), nil))

		a.send(t, http.MethodPost, "/confirm?method=password", cookie)

		record, err := a.store.Find(t.Context(), hashing.SHA256([]byte(cookie.Value)))
		require.NoError(t, err)
		assert.Equal(t, "passkey", record.AuthMethod)
		assert.Equal(t, "password", record.ConfirmedMethod)
	})

	t.Run("stores a visitor with no sign-in record", func(t *testing.T) {
		t.Parallel()
		a := newApp(t, db, session.Config{})
		cookie := a.sessionCookie(a.send(t, http.MethodPost, "/put?key=color&value=blue", nil))

		record, err := a.store.Find(t.Context(), hashing.SHA256([]byte(cookie.Value)))

		require.NoError(t, err)
		assert.Empty(t, record.AuthMethod)
		assert.Empty(t, record.AuthMethods)
		assert.Empty(t, record.DeviceID)
		assert.Empty(t, record.Country)
	})

	t.Run("keeps the connection and domain a sign-in came through until the next login", func(t *testing.T) {
		t.Parallel()
		a := newApp(t, db, session.Config{})
		connection := ids.New()
		s, err := a.manager.Start(newRequest(t))
		require.NoError(t, err)

		s.Login(ids.New(), "sso")
		s.SetSignInContext(connection, "acme.example")
		record := s.Record()
		require.NoError(t, a.store.Save(t.Context(), record))

		stored, err := a.store.Find(t.Context(), record.TokenHash)
		require.NoError(t, err)
		assert.Equal(t, connection, stored.ConnectionID)
		assert.Equal(t, "acme.example", stored.AuthDomain)

		s.Login(ids.New(), "password")
		assert.Equal(t, ids.Nil(), s.Record().ConnectionID)
		assert.Empty(t, s.Record().AuthDomain)
	})

	t.Run("is copied out by record", func(t *testing.T) {
		t.Parallel()
		a := newApp(t, db, session.Config{})
		s, err := a.manager.Start(newRequest(t))
		require.NoError(t, err)
		s.Login(ids.New(), "password", "totp")

		record := s.Record()
		record.AuthMethods[0] = "passkey"
		methods := s.AuthMethods()
		methods[1] = "sms"

		assert.Equal(t, []string{"password", "totp"}, s.AuthMethods())
	})
}

func TestSignInConstraints(t *testing.T) {
	t.Parallel()

	db := openDB(t)
	store := session.NewPostgresStore(db, session.DefaultTable)

	t.Run("refuses an auth method on a visitor's session", func(t *testing.T) {
		t.Parallel()
		r := record(ids.Nil())
		r.AuthMethod = "password"

		require.Error(t, store.Save(t.Context(), r))
	})

	t.Run("refuses a method name that is not a lowercase identifier", func(t *testing.T) {
		t.Parallel()
		r := record(ids.New())
		r.AuthMethod = "Pass Word"

		require.Error(t, store.Save(t.Context(), r))
	})

	t.Run("refuses a country that is not a two-letter code", func(t *testing.T) {
		t.Parallel()
		r := record(ids.New())
		r.Country = "Netherlands"

		require.Error(t, store.Save(t.Context(), r))
	})

	t.Run("stores a login that named no method", func(t *testing.T) {
		t.Parallel()
		r := record(ids.New())

		require.NoError(t, store.Save(t.Context(), r))
		found, err := store.Find(t.Context(), r.TokenHash)
		require.NoError(t, err)
		assert.Equal(t, []string{}, found.AuthMethods)
	})
}

func TestConfigure(t *testing.T) {
	t.Parallel()

	t.Run("replaces the settings and fills in the defaults", func(t *testing.T) {
		t.Parallel()
		a := newApp(t, openDB(t), session.Config{})

		a.manager.Configure(session.Config{Cookie: "sid", Insecure: true})

		cookie := a.sessionCookie(a.send(t, http.MethodPost, "/put?key=color&value=blue", nil))
		require.NotNil(t, cookie)
		assert.Equal(t, "sid", cookie.Name)
		assert.False(t, cookie.Secure)
		assert.Equal(t, int((30 * 24 * time.Hour).Seconds()), cookie.MaxAge)
	})
}

/*
record returns a session record of the user, ready to save, with a
token digest of its own.
*/
func record(user ids.UUID) session.Record {
	now := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	id := ids.NewV7At(now)

	return session.Record{
		ID: id, TokenHash: hashing.SHA256([]byte(id.String())), UserID: user, LastActivityAt: now,
		ExpiresAt: now.Add(time.Hour), CreatedAt: now, UpdatedAt: now,
	}
}
