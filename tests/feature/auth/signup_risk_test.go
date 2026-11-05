package auth_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/config"
	"github.com/dimmihq/dimmi/resources/data"
	"github.com/dimmihq/dimmi/tests"
)

/*
throwaway is an address at a domain on the embedded list.
*/
const throwaway = "ada@mailinator.com"

func TestSignupRisk(t *testing.T) {
	t.Parallel()

	t.Run("refuses a throwaway mailbox when the server blocks them", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_AUTH__SIGNUP__MODE=open", "DIMMI_AUTH__DISPOSABLE_EMAIL=block"))

		tc.Client().PostJSON("/api/v1/users", registration(throwaway)).
			AssertValidationError("email").AssertJSONPath("code", "disposable_email")
		tc.Client().PostJSON("/api/v1/users", registration("ada@inbox.mailinator.com")).AssertValidationError("email")
		tc.Table("users").AssertMissing(testkit.Row{"email": throwaway})
	})

	t.Run("lets a throwaway mailbox in marked when the server flags them", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_AUTH__SIGNUP__MODE=open", "DIMMI_AUTH__DISPOSABLE_EMAIL=flag"))

		tc.Client().PostJSON("/api/v1/users", registration(throwaway)).AssertCreated()

		var flags []string
		require.NoError(t, tc.Operator().QueryRow(t.Context(),
			"SELECT risk_flags FROM users WHERE email = $1", throwaway).Scan(&flags))
		assert.Equal(t, []string{appauth.RiskDisposableEmail}, flags)
	})

	t.Run("judges nothing on a server that lets every mailbox in", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_AUTH__SIGNUP__MODE=open", "DIMMI_AUTH__DISPOSABLE_EMAIL=off"))

		tc.Client().PostJSON("/api/v1/users", registration(throwaway)).AssertCreated()
	})

	t.Run("marks the signups of a network that makes too many in a day", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		domains, err := data.DisposableDomains()
		require.NoError(t, err)
		risk := &appauth.SignupRisk{Disposable: domains, Mode: config.DisposableFlag, Limiter: tc.App.Limiter}

		for i := range 20 {
			marks, err := risk.Assess(t.Context(), "ada@example.com", fmt.Sprintf("203.0.113.%d", i+1))
			require.NoError(t, err)
			assert.Empty(t, marks)
		}

		marks, err := risk.Assess(t.Context(), throwaway, "203.0.113.200")
		require.NoError(t, err)
		assert.Equal(t, []string{appauth.RiskDisposableEmail, appauth.RiskSignupVelocity}, marks)

		marks, err = risk.Assess(t.Context(), "ada@example.com", "2001:db8::1")
		require.NoError(t, err)
		assert.Empty(t, marks)

		marks, err = risk.Assess(t.Context(), "not an address", "nonsense")
		require.NoError(t, err)
		assert.Empty(t, marks)
	})

	t.Run("refuses to boot with an unknown treatment", func(t *testing.T) {
		t.Parallel()

		_, _, err := tests.Boot(t, tests.WithEnv("DIMMI_AUTH__DISPOSABLE_EMAIL=maybe"))
		require.ErrorContains(t, err, "auth.disposable_email")
	})
}
