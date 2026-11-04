package user_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/auth/twofactor"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/factors"
	authfactors "github.com/dimmihq/dimmi/app/auth/factors"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
outage breaks one piece of the test's storage.
*/
type outage func(tc *tests.TestCase)

func broken(table string) outage {
	return func(tc *tests.TestCase) { tc.BreakTable(table) }
}

func failing(table string, kinds ...string) outage {
	return func(tc *tests.TestCase) { tc.FailWrites(table, kinds...) }
}

/*
pendingAuthenticator adds an authenticator for the owner and returns it
with the code it shows now.
*/
func pendingAuthenticator(t *testing.T, tc *tests.TestCase) (factors.NewAuthFactor, string) {
	t.Helper()

	added, err := tc.App.Actions.AddAuthFactor.Handle(t.Context(), factors.AddAuthFactorInput{
		User: tc.Owner, Type: authfactors.TOTPType,
	})
	require.NoError(t, err)

	code, err := twofactor.Code(added.Setup.Secret, tc.Clock.Now())
	require.NoError(t, err)

	return added, code
}

/*
protected returns a new account with two-factor authentication on.
*/
func protected(t *testing.T, tc *tests.TestCase) factories.TwoFactorAccount {
	t.Helper()

	account, err := factories.WithTwoFactor(t.Context(), tc.App, tc.CreateUser())
	require.NoError(t, err)

	return account
}

func TestAuthFactorStorageFailures(t *testing.T) {
	t.Parallel()

	t.Run("adds no factor when its storage fails", func(t *testing.T) {
		t.Parallel()

		for name, out := range map[string]outage{
			"users":             broken("users"),
			"factors":           broken("user_auth_factors"),
			"replacing":         failing("user_auth_factors", "DELETE"),
			"storing":           failing("user_auth_factors", "INSERT"),
			"keys":              broken("encryption_keys"),
			"rate limit counts": func(tc *tests.TestCase) { tc.BreakValkey() },
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				tc := tests.New(t)
				out(tc)

				_, err := tc.App.Actions.AddAuthFactor.Handle(t.Context(), factors.AddAuthFactorInput{
					User: tc.Owner, Type: authfactors.TOTPType,
				})

				require.Error(t, err)
			})
		}
	})

	t.Run("activates nothing when its storage fails", func(t *testing.T) {
		t.Parallel()

		for name, out := range map[string]outage{
			"users":             broken("users"),
			"factors":           broken("user_auth_factors"),
			"activating":        failing("user_auth_factors", "UPDATE"),
			"recovery codes":    failing("user_auth_factors", "DELETE"),
			"marking the user":  failing("users", "UPDATE"),
			"logging":           failing("security_events", "INSERT"),
			"mailing":           failing("river_job", "INSERT"),
			"rate limit counts": func(tc *tests.TestCase) { tc.BreakValkey() },
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				tc := tests.New(t)
				added, code := pendingAuthenticator(t, tc)
				out(tc)

				_, err := tc.App.Actions.ActivateAuthFactor.Handle(t.Context(), factors.ActivateAuthFactorInput{
					User: tc.Owner, FactorID: added.Factor.ID, Code: code,
				})

				require.Error(t, err)
			})
		}
	})

	t.Run("reports the last wrong code when the pending factor cannot be deleted", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		added, _ := pendingAuthenticator(t, tc)
		tc.FailWrites("user_auth_factors", "DELETE")

		var err error
		for range 5 {
			_, err = tc.App.Actions.ActivateAuthFactor.Handle(t.Context(), factors.ActivateAuthFactorInput{
				User: tc.Owner, FactorID: added.Factor.ID, Code: "000000",
			})
		}

		require.Error(t, err)
		require.NotErrorIs(t, err, exceptions.ErrInvalidCode)
		tc.Table("user_auth_factors").AssertCount(1)
	})

	t.Run("turns two-factor on without recovery codes while they are disabled", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_AUTH__FACTORS__ENABLED=totp"))
		added, code := pendingAuthenticator(t, tc)

		activated, err := tc.App.Actions.ActivateAuthFactor.Handle(t.Context(), factors.ActivateAuthFactorInput{
			User: tc.Owner, FactorID: added.Factor.ID, Code: code,
		})

		require.NoError(t, err)
		require.NotNil(t, activated.RecoveryCodes)
		assert.Empty(t, activated.RecoveryCodes.Setup.Codes)
		tc.Table("user_auth_factors").AssertCount(0, testkit.Row{"type": authfactors.RecoveryCodeType})

		_, err = tc.App.Actions.RegenerateRecoveryCodes.Handle(t.Context(), factors.RegenerateRecoveryCodesInput{
			User: tc.Owner,
		})
		require.ErrorIs(t, err, exceptions.ErrFactorTypeNotAvailable)
	})

	t.Run("changes no factor when its storage fails", func(t *testing.T) {
		t.Parallel()
		name := "Work phone"

		for label, in := range map[string]struct {
			out   outage
			input factors.UpdateAuthFactorInput
		}{
			"locking":    {broken("user_auth_factors"), factors.UpdateAuthFactorInput{Preferred: true}},
			"preferring": {failing("user_auth_factors", "UPDATE"), factors.UpdateAuthFactorInput{Preferred: true}},
			"renaming":   {failing("user_auth_factors", "UPDATE"), factors.UpdateAuthFactorInput{Name: &name}},
		} {
			t.Run(label, func(t *testing.T) {
				t.Parallel()
				tc := tests.New(t)
				account := protected(t, tc)
				in.out(tc)
				in.input.User, in.input.FactorID = account.User, account.TOTP.ID

				_, err := tc.App.Actions.UpdateAuthFactor.Handle(t.Context(), in.input)

				require.Error(t, err)
			})
		}
	})

	t.Run("lists no factor when they cannot be read", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		tc.BreakTable("user_auth_factors")

		_, err := tc.App.Actions.ListAuthFactors.Handle(t.Context(), tc.Owner.ID)

		require.Error(t, err)
	})
}

func TestTwoFactorOffStorageFailures(t *testing.T) {
	t.Parallel()

	t.Run("keeps the last protecting factor when turning two-factor off fails", func(t *testing.T) {
		t.Parallel()

		for name, out := range map[string]outage{
			"users":            broken("users"),
			"factors":          broken("user_auth_factors"),
			"workspaces":       broken("workspaces"),
			"deleting":         failing("user_auth_factors", "DELETE"),
			"marking the user": failing("users", "UPDATE"),
			"logging":          failing("security_events", "INSERT"),
			"mailing":          failing("river_job", "INSERT"),
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				tc := tests.New(t)
				account := protected(t, tc)
				out(tc)

				err := tc.App.Actions.RemoveAuthFactor.Handle(t.Context(), factors.RemoveAuthFactorInput{
					User: account.User, FactorID: account.TOTP.ID,
				})

				require.Error(t, err)
			})
		}
	})

	t.Run("keeps every factor when removing them all fails", func(t *testing.T) {
		t.Parallel()

		for name, out := range map[string]outage{
			"users":      broken("users"),
			"workspaces": broken("workspaces"),
			"logging":    failing("security_events", "INSERT"),
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				tc := tests.New(t)
				account := protected(t, tc)
				out(tc)

				_, err := tc.App.Actions.RemoveAllAuthFactors.Handle(t.Context(), factors.RemoveAllAuthFactorsInput{
					UserID: account.User.ID,
				})

				require.Error(t, err)
			})
		}
	})

	t.Run("keeps an unfinished factor when it cannot be deleted", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		pendingAuthenticator(t, tc)
		tc.FailWrites("user_auth_factors", "DELETE")

		_, err := tc.App.Actions.RemoveAllAuthFactors.Handle(t.Context(), factors.RemoveAllAuthFactorsInput{
			UserID: tc.Owner.ID,
		})

		require.Error(t, err)
		tc.Table("user_auth_factors").AssertCount(1)
	})

	t.Run("keeps the recovery codes when new ones cannot be stored", func(t *testing.T) {
		t.Parallel()

		for name, out := range map[string]outage{
			"users":    broken("users"),
			"deleting": failing("user_auth_factors", "DELETE"),
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				tc := tests.New(t)
				account := protected(t, tc)
				out(tc)

				_, err := tc.App.Actions.RegenerateRecoveryCodes.Handle(t.Context(),
					factors.RegenerateRecoveryCodesInput{User: account.User})

				require.Error(t, err)
			})
		}
	})

	t.Run("keeps a passkey when removing it beside another fails", func(t *testing.T) {
		t.Parallel()

		for name, out := range map[string]outage{
			"deleting":   failing("user_auth_factors", "DELETE"),
			"preferring": failing("user_auth_factors", "UPDATE"),
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				tc := tests.New(t)
				laptop, res := addPasskey(t, tc, tc.ActingAsOwner(), "Laptop")
				browser := rotated(tc, res)
				addPasskey(t, tc, browser, "Phone")
				out(tc)

				browser.Delete(factorsPath + "/" + laptop.id).AssertStatus(fhttp.StatusInternalServerError)
				tc.Table("user_auth_factors").AssertHas(testkit.Row{"type": "passkey", "name": "Laptop"})
			})
		}
	})
}

func TestAuthFactorDrivers(t *testing.T) {
	t.Parallel()

	t.Run("refuses to boot with a kind no driver registers", func(t *testing.T) {
		t.Parallel()

		_, _, err := tests.Boot(t, tests.WithEnv("DIMMI_AUTH__FACTORS__ENABLED=totp,sms"))

		require.ErrorIs(t, err, authfactors.ErrUnknownFactor)
	})

	t.Run("names a passkey added without a name", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.ActingAsOwner().PostJSON(factorsPath, testkit.JSON{"type": "passkey"}).AssertCreated().
			AssertJSONPath("data.name", "Passkey")
	})

	t.Run("refuses to finish a passkey whose registration was lost", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser := tc.ActingAsOwner()
		id, options := startPasskey(browser, "Laptop")
		answer, err := newDevice(tc).Create(options)
		require.NoError(t, err)
		_, err = tc.Operator().Exec(t.Context(), "UPDATE user_auth_factors SET credential = '{}'")
		require.NoError(t, err)

		browser.PatchJSON(factorsPath+"/"+id, testkit.JSON{"credential": answer}).
			AssertValidationError("credential")
	})

	t.Run("refuses a passkey answer when its stored credential was lost", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		laptop, _ := addPasskey(t, tc, tc.ActingAsOwner(), "Laptop")
		browser, challenge := signInWithPassword(tc, tc.Owner.Email)
		signed := signPasskey(t, challenge, laptop.key)
		_, err := tc.Operator().Exec(t.Context(),
			"UPDATE user_auth_factors SET credential = '{}' WHERE type = 'passkey'")
		require.NoError(t, err)

		answerPasskey(browser, signed).AssertValidationError("credential")
	})
}
