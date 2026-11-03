package user_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/tests"
)

/*
purge runs the scheduled purge of expired sign-in records now.
*/
func purge(t *testing.T, tc *tests.TestCase) {
	t.Helper()

	require.NoError(t, tc.App.Queue.Push(t.Context(), jobs.PurgeExpiredAuthRecordsArgs{}))
	tc.Drain()
}

func TestPurgeExpiredAuthRecords(t *testing.T) {
	t.Parallel()

	t.Run("deletes records that ran out and keeps the ones still in use", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		other := tc.CreateUser()
		stale := tc.ActingAsOwner()
		addAuthenticator(t, stale)
		tc.Client().PostJSON("/api/v1/magic-links", testkit.JSON{"email": tc.Owner.Email}).
			AssertStatus(fhttp.StatusAccepted)
		tc.Client().PostJSON("/api/v1/password-resets", testkit.JSON{"email": tc.Owner.Email}).
			AssertStatus(fhttp.StatusAccepted)
		tc.Table("user_auth_factors").AssertCount(1, testkit.Row{"status": "pending"})
		tc.Table("one_time_tokens").AssertCount(1, testkit.Row{"email": tc.Owner.Email})
		tc.Table("password_reset_tokens").AssertCount(1, testkit.Row{"user_id": tc.Owner.ID})

		tc.Clock.Advance(tc.App.Config.Auth.Sessions.IdleLifetime + time.Hour)
		fresh := tc.ActingAsOwner()
		addAuthenticator(t, fresh)
		tc.Client().PostJSON("/api/v1/magic-links", testkit.JSON{"email": other.Email}).
			AssertStatus(fhttp.StatusAccepted)
		tc.Client().PostJSON("/api/v1/password-resets", testkit.JSON{"email": other.Email}).
			AssertStatus(fhttp.StatusAccepted)

		purge(t, tc)

		tc.Table("user_auth_factors").AssertCount(1, testkit.Row{"status": "pending"})
		tc.Table("one_time_tokens").AssertCount(0, testkit.Row{"email": tc.Owner.Email}).
			AssertCount(1, testkit.Row{"email": other.Email})
		tc.Table("password_reset_tokens").AssertCount(0, testkit.Row{"user_id": tc.Owner.ID}).
			AssertCount(1, testkit.Row{"user_id": other.ID})
		tc.Table("sessions").AssertCount(1, testkit.Row{"user_id": tc.Owner.ID})
		stale.GetJSON("/api/v1/me").AssertUnauthorized()
		fresh.GetJSON("/api/v1/me").AssertOK()
	})

	t.Run("keeps an expired link for a day, so opening it still says it ran out", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		tc.Client().PostJSON("/api/v1/magic-links", testkit.JSON{"email": tc.Owner.Email}).
			AssertStatus(fhttp.StatusAccepted)
		tc.Clock.Advance(jobs.SpentTokenRetention - time.Hour)

		purge(t, tc)

		tc.Table("one_time_tokens").AssertCount(1, testkit.Row{"email": tc.Owner.Email})
	})
}
