package socialite_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/socialite"
	"github.com/dimmihq/dimmi/framework/socialite/drivertest"
)

func newFakeHarness(t *testing.T) drivertest.Harness {
	t.Helper()

	m := socialite.NewManager(socialite.Config{
		Providers:  map[string]config.Section{"google": {"driver": "google"}},
		AllowFakes: true,
	}, nil)
	fake := m.Fake()

	provider, err := m.Driver("google")
	require.NoError(t, err)

	user := socialite.User{
		Provider:      "google",
		ID:            "108",
		Email:         "ada@example.com",
		EmailVerified: true,
		Name:          "Ada Lovelace",
		AvatarURL:     "https://example.com/ada.png",
	}

	return drivertest.Harness{
		Provider: provider,
		User:     user,
		Approve: func(t *testing.T, authURL string) string {
			t.Helper()

			callback, err := fake.Approve(authURL, user)
			require.NoError(t, err)

			return callback
		},
	}
}

func TestFake(t *testing.T) {
	t.Parallel()

	drivertest.Run(t, newFakeHarness)

	t.Run("refuses a fake unless fakes are allowed", func(t *testing.T) {
		t.Parallel()
		m := socialite.NewManager(socialite.Config{}, nil)

		assert.PanicsWithValue(t, socialite.ErrFakesNotAllowed, func() { m.Fake() })
	})

	t.Run("approves only urls it built", func(t *testing.T) {
		t.Parallel()
		m := socialite.NewManager(socialite.Config{AllowFakes: true}, nil)
		fake := m.Fake()

		_, err := fake.Approve("https://accounts.example.com/authorize?state=x", socialite.User{})
		require.ErrorIs(t, err, socialite.ErrProvider)

		_, err = fake.Approve("https://socialite.test/google/authorize?redirect_uri=x", socialite.User{})
		require.ErrorIs(t, err, socialite.ErrProvider)
	})

	t.Run("knows only the access tokens it issued", func(t *testing.T) {
		t.Parallel()
		h := newFakeHarness(t)

		_, err := h.Provider.User(t.Context(), socialite.Token{AccessToken: "made-up"})

		require.ErrorIs(t, err, socialite.ErrProvider)
	})
}
