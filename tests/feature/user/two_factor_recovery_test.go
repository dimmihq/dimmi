package user_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

func TestUserDisableTwoFactorCommand(t *testing.T) {
	t.Parallel()

	t.Run("lets an operator turn two-factor off for a locked-out user", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		account, err := factories.WithTwoFactor(t.Context(), tc.App, tc.CreateUser())
		require.NoError(t, err)

		code, out := tc.Command("user:disable-2fa", strings.ToUpper(account.User.Email))

		assert.Equal(t, 0, code, out)
		assert.Contains(t, out, "is off for "+account.User.Email)
		tc.Table("user_auth_factors").AssertCount(0, testkit.Row{"user_id": account.User.ID})
		tests.LastSent[appmail.TwoFactorDisabledMail](tc, fake)
		tc.Client().PostJSON("/api/v1/sessions", testkit.JSON{
			"grant": "password", "email": account.User.Email, "password": factories.Password,
		}).AssertCreated()

		code, out = tc.Command("user:disable-2fa", account.User.Email)
		assert.Equal(t, 0, code)
		assert.Contains(t, out, "already off")
	})

	t.Run("fails for an email without an account", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		code, out := tc.Command("user:disable-2fa", "nobody@example.com")

		assert.Equal(t, 1, code)
		assert.Contains(t, out, "no account has this email")
	})
}

func TestAuthFactorSealing(t *testing.T) {
	t.Parallel()

	t.Run("never opens a secret copied onto another factor", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		victim, err := factories.WithTwoFactor(t.Context(), tc.App, tc.CreateUser())
		require.NoError(t, err)
		attacker, err := factories.WithTwoFactor(t.Context(), tc.App, tc.CreateUser())
		require.NoError(t, err)
		_, err = tc.App.DB.Exec(t.Context(),
			"UPDATE user_auth_factors SET secret_encrypted = $1 WHERE id = $2",
			[]byte(victim.TOTP.SecretEncrypted), attacker.TOTP.ID)
		require.NoError(t, err)

		res := tc.Client().PostJSON("/api/v1/sessions", testkit.JSON{
			"grant": "password", "email": attacker.User.Email, "password": factories.Password,
		}).AssertOK()

		tc.Client().WithCookie(res.Cookie(tc.App.Cookies.TwoFactorName())).PostJSON("/api/v1/sessions",
			testkit.JSON{"grant": "two_factor", "method": "totp", "code": codeFor(t, tc, victim.Secret)}).
			AssertStatus(fhttp.StatusInternalServerError)
		tc.Table("sessions").AssertCount(0, testkit.Row{"user_id": attacker.User.ID})
	})
}
