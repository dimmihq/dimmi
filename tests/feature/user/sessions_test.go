package user_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/session"
	"github.com/dimmihq/dimmi/framework/testkit"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

func TestSessionIndex(t *testing.T) {
	t.Parallel()

	t.Run("lists the user's browsers with how each signed in, the current one marked", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		laptop := tc.ActingAsOwner()
		phone := tc.ActingAsOwner().WithHeader("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X)")
		current := phone.GetJSON("/api/v1/me").String("data.auth.session_id")
		tc.ActingAs(tc.CreateUser())

		res := laptop.GetJSON("/api/v1/sessions").AssertOK()

		res.AssertJSONPath("data.#", 2).
			AssertJSONPath("data.0.object", "session").
			AssertJSONPath("data.0.kind", "browser").
			AssertJSONPath("data.0.auth_method", "password").
			AssertJSONPath("data.0.auth_methods", []string{"password"}).
			AssertJSONPath("data.0.two_factor_used", false).
			AssertJSONPath("data.0.client", nil).
			AssertJSONMissing("data.0.device_id").
			AssertJSONPath("meta.next_cursor", nil)
		assert.ElementsMatch(t, []bool{true, false}, []bool{
			res.String("data.0.current") == "true", res.String("data.1.current") == "true",
		})
		assert.NotEqual(t, current, laptop.GetJSON("/api/v1/me").String("data.auth.session_id"))
	})

	t.Run("refuses an access token, which is not the user themselves", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.ActingAsToken(tc.Workspace.Owner, "*").GetJSON("/api/v1/sessions").
			AssertForbidden().
			AssertJSONPath("code", "first_party_only")
	})

	t.Run("refuses a caller without a session", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.Client().GetJSON("/api/v1/sessions").AssertUnauthorized().AssertJSONPath("code", "unauthenticated")
	})
}

func TestSessionDevices(t *testing.T) {
	t.Parallel()

	t.Run("names each browser for people from what it says about itself", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		agents := map[string]string{
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) " +
				"Chrome/120.0.0.0 Safari/537.36": "Chrome on MacOS",
			"curl/8.4.0": "",
		}

		for agent, label := range agents {
			client := tc.Client().WithHeader("User-Agent", agent)
			signedIn := client.PostJSON("/api/v1/sessions", testkit.JSON{
				"grant": "password", "email": tc.Owner.Email, "password": factories.Password,
			})
			browser := client.WithCookie(signedIn.Cookie(tc.App.Session.CookieName()))

			sessions := browser.GetJSON("/api/v1/sessions").AssertOK()

			assert.Equal(t, label, sessions.String(`data.#(current==true).device_label`), agent)
		}
	})
}

func TestSessionDestroy(t *testing.T) {
	t.Parallel()

	t.Run("signs the browser out and clears its cookie", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser := tc.ActingAsOwner()

		res := browser.Delete("/api/v1/sessions/current").AssertNoContent()

		assert.Negative(t, res.Cookie(tc.App.Session.CookieName()).MaxAge)
		browser.GetJSON("/api/v1/me").AssertUnauthorized()
		tc.Table("sessions").AssertCount(0, testkit.Row{"user_id": tc.Owner.ID})
		tc.Table("security_events").AssertHas(testkit.Row{"user_id": tc.Owner.ID, "type": "logout.succeeded"})
	})

	t.Run("ends another of the user's sessions, leaving the current one", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		lost := tc.ActingAsOwner()
		lostID := lost.GetJSON("/api/v1/me").String("data.auth.session_id")
		browser := tc.ActingAsOwner()

		browser.Delete("/api/v1/sessions/" + lostID).AssertNoContent()

		lost.GetJSON("/api/v1/me").AssertUnauthorized()
		browser.GetJSON("/api/v1/me").AssertOK()
		tc.Table("security_events").AssertHas(testkit.Row{"user_id": tc.Owner.ID, "type": "session.revoked"})
	})

	t.Run("ends the current session when named by its own id", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser := tc.ActingAsOwner()
		own := browser.GetJSON("/api/v1/me").String("data.auth.session_id")

		browser.Delete("/api/v1/sessions/" + own).AssertNoContent()

		browser.GetJSON("/api/v1/me").AssertUnauthorized()
	})

	t.Run("does not find another user's session, a made-up one or an id of another kind", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		other := tc.ActingAs(tc.CreateUser())
		otherID := other.GetJSON("/api/v1/me").String("data.auth.session_id")
		browser := tc.ActingAsOwner()

		for _, id := range []string{
			otherID, models.PublicID(models.PrefixSession, ids.New()),
			models.PublicID(models.PrefixInvitation, ids.New()), "not-an-id",
		} {
			browser.Delete("/api/v1/sessions/"+id).AssertNotFound().AssertJSONPath("code", "not_found")
		}

		other.GetJSON("/api/v1/me").AssertOK()
	})
}

func TestSessionDestroyMany(t *testing.T) {
	t.Parallel()

	t.Run("signs the user out everywhere else", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		others := []*testkit.Client{tc.ActingAsOwner(), tc.ActingAsOwner()}
		browser := tc.ActingAsOwner()

		browser.Delete("/api/v1/sessions").AssertNoContent()

		for _, other := range others {
			other.GetJSON("/api/v1/me").AssertUnauthorized()
		}
		browser.GetJSON("/api/v1/me").AssertOK()
		tc.Table("security_events").AssertHas(testkit.Row{"user_id": tc.Owner.ID, "type": "sessions.revoked_others"})
	})

	t.Run("asks the user to confirm who they are once the sign-in is no longer recent", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		other := tc.ActingAsOwner()
		browser := tc.ActingAsOwner()
		tc.Clock.Advance(16 * time.Minute)

		browser.Delete("/api/v1/sessions").
			AssertStatus(423).
			AssertJSONPath("code", "password_confirmation_required")

		other.GetJSON("/api/v1/me").AssertOK()
	})
}

func TestSessionLifetime(t *testing.T) {
	t.Parallel()

	t.Run("ends a session left idle for fourteen days", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser := tc.ActingAsOwner()

		tc.Clock.Advance(14*24*time.Hour - time.Second)
		browser.GetJSON("/api/v1/me").AssertOK()
		tc.Clock.Advance(14 * 24 * time.Hour)

		browser.GetJSON("/api/v1/me").AssertUnauthorized()
	})

	t.Run("ends any session thirty days after sign-in, however active", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser := tc.ActingAsOwner()

		for range 3 {
			tc.Clock.Advance(10*24*time.Hour - time.Minute)
			browser.GetJSON("/api/v1/me").AssertOK()
		}
		tc.Clock.Advance(time.Hour)

		browser.GetJSON("/api/v1/me").AssertUnauthorized()
	})

	t.Run("names its cookies with the __Host- prefix when served over https", func(t *testing.T) {
		t.Parallel()
		secure := tests.New(t, tests.WithEnv("DIMMI_APP__URL=https://app.example.com"))
		plain := tests.New(t)

		assert.Equal(t, "__Host-dimmi_session", secure.App.Session.CookieName())
		assert.Equal(t, "__Host-dimmi_device", secure.App.Cookies.DeviceName())
		assert.Equal(t, "dimmi_session", plain.App.Session.CookieName())
		secure.ActingAsOwner().GetJSON("/api/v1/me").AssertOK()
	})
}

func TestSessionStore(t *testing.T) {
	t.Parallel()

	t.Run("stores how a session signed in as columns and reads it back", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		store := session.NewPostgresStore(tc.App.DB, session.DefaultTable)
		record := signedIn(tc.Clock.Now(), tc.Owner.ID)
		record.Data = map[string]string{"theme": "dark"}

		require.NoError(t, store.Save(t.Context(), record))
		found, err := store.Find(t.Context(), record.TokenHash)
		require.NoError(t, err)

		assert.Equal(t, "dark", found.Data["theme"])
		assert.Equal(t, appauth.SignIn{
			Method: "password", Methods: []string{"password", "totp"}, DeviceID: "device", ConfirmedMethod: "totp",
		}, appauth.SignInFrom(found))
		assert.True(t, appauth.SignInFrom(found).UsedSecondFactor())
		tc.Table("sessions").AssertHas(testkit.Row{"id": record.ID, "auth_method": "password", "device_id": "device"})
		listed := `data.#(id=="` + models.PublicID(models.PrefixSession, record.ID) + `")`
		tc.ActingAsOwner().GetJSON("/api/v1/sessions").AssertOK().
			AssertJSONPath(listed+".two_factor_used", true).
			AssertJSONPath(listed+".country", "NL")
	})

	t.Run("refuses a session without a signed-in user", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		store := session.NewPostgresStore(tc.App.DB, session.DefaultTable)
		record := signedIn(tc.Clock.Now(), ids.Nil())
		record.AuthMethod, record.AuthMethods, record.ConfirmedMethod = "", nil, ""

		require.Error(t, store.Save(t.Context(), record))
	})

	t.Run("refuses a signed-in session that records no auth method", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		store := session.NewPostgresStore(tc.App.DB, session.DefaultTable)
		record := signedIn(tc.Clock.Now(), tc.Owner.ID)
		record.AuthMethod = ""

		require.Error(t, store.Save(t.Context(), record))
	})
}

/*
signedIn returns a session record of the user, signed in with a
password and a second factor on a known device, ready to save.
*/
func signedIn(now time.Time, user ids.UUID) session.Record {
	return session.Record{
		ID: ids.NewV7At(now), TokenHash: []byte("digest " + ids.New().String()), UserID: user,
		AuthMethod: "password", AuthMethods: []string{"password", "totp"}, ConfirmedMethod: "totp",
		DeviceID: "device", Country: "NL", AuthenticatedAt: now, ConfirmedAt: now, LastActivityAt: now,
		ExpiresAt: now.Add(time.Hour), CreatedAt: now, UpdatedAt: now,
	}
}
