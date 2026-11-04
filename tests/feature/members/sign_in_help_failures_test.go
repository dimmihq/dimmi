package members_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

func TestSignInHelpFailures(t *testing.T) {
	t.Parallel()

	t.Run("mails no link when the request cannot be recorded", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, member := protected(t, tc, enums.RoleMember)
		owner := in(tc, tc.Owner)
		tc.FailWrites("audit_logs", "INSERT")

		owner.PostJSON(resetPath(member), testkit.JSON{}).AssertStatus(fhttp.StatusInternalServerError)

		tc.Table("one_time_tokens").AssertCount(0, testkit.Row{"purpose": "two_factor_reset"})
	})

	t.Run("mails no link when the teammate's log cannot be written", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, member := protected(t, tc, enums.RoleMember)
		owner := in(tc, tc.Owner)
		tc.FailWrites("security_events", "INSERT")

		owner.PostJSON(resetPath(member), testkit.JSON{}).AssertStatus(fhttp.StatusInternalServerError)

		tc.Table("one_time_tokens").AssertCount(0, testkit.Row{"purpose": "two_factor_reset"})
	})

	t.Run("keeps the second factor when the reset cannot be recorded", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		teammate, member := protected(t, tc, enums.RoleMember)
		id, token := mailReset(t, tc, in(tc, tc.Owner), member)
		tc.FailWrites("audit_logs", "INSERT")

		redeem(tc.Client(), id, testkit.JSON{"token": token, "password": factories.Password}).
			AssertStatus(fhttp.StatusInternalServerError)

		tc.Table("users").AssertMissing(testkit.Row{"id": teammate.ID, "two_factor_confirmed_at": nil})
		tc.Table("one_time_tokens").AssertHas(testkit.Row{"purpose": "two_factor_reset", "consumed_at": nil})
	})

	t.Run("keeps the second factor when the account's log cannot be written", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		teammate, member := protected(t, tc, enums.RoleMember)
		id, token := mailReset(t, tc, in(tc, tc.Owner), member)
		tc.FailWrites("security_events", "INSERT")

		redeem(tc.Client(), id, testkit.JSON{"token": token, "password": factories.Password}).
			AssertStatus(fhttp.StatusInternalServerError)

		tc.Table("users").AssertMissing(testkit.Row{"id": teammate.ID, "two_factor_confirmed_at": nil})
	})

	t.Run("refuses a link once its account is deleted", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		teammate, member := protected(t, tc, enums.RoleMember)
		id, token := mailReset(t, tc, in(tc, tc.Owner), member)
		require.NoError(t, factories.DeleteAccount(t.Context(), tc.App, teammate))

		redeem(tc.Client(), id, testkit.JSON{"token": token}).
			AssertJSONPath("code", "invalid_two_factor_reset_token")
	})

	t.Run("answers busy while counters cannot be read", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, member := join(t, tc, enums.RoleMember)
		owner := in(tc, tc.Owner)
		tc.BreakValkey()

		owner.Delete(lockPath(memberID(member))).AssertStatus(fhttp.StatusServiceUnavailable)
		redeem(tc.Client(), "ott_nope", testkit.JSON{"token": "x"}).AssertStatus(fhttp.StatusServiceUnavailable)
	})
}
