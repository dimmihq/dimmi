package workspaces_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/tests"
)

func TestPersonalAccessTokenFailures(t *testing.T) {
	t.Parallel()

	t.Run("mints nothing when the role a token is minted under cannot be remembered", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser := in(tc, tc.Owner)
		tc.FailWrites("personal_access_tokens", "UPDATE")

		browser.PostJSON(patPath, testkit.JSON{"name": "CI", "abilities": []string{"members:read"}}).AssertStatus(500)

		tc.Table("personal_access_tokens").AssertCount(0)
	})

	t.Run("mints nothing when a write it needs fails", func(t *testing.T) {
		t.Parallel()

		for table, kinds := range map[string][]string{
			"personal_access_tokens": {"INSERT"},
			"security_events":        {"INSERT"},
			"river_job":              {"INSERT"},
		} {
			t.Run(table, func(t *testing.T) {
				t.Parallel()
				tc := tests.New(t)
				browser := in(tc, tc.Owner)
				tc.FailWrites(table, kinds...)

				browser.PostJSON(patPath, testkit.JSON{"name": "CI", "abilities": []string{"*"}}).AssertStatus(500)
				tc.Table("personal_access_tokens").AssertCount(0)
			})
		}
	})

	t.Run("keeps a token working when revoking or renaming it fails", func(t *testing.T) {
		t.Parallel()

		for table, kinds := range map[string][]string{
			"personal_access_tokens": {"UPDATE"},
			"security_events":        {"INSERT"},
		} {
			t.Run(table, func(t *testing.T) {
				t.Parallel()
				tc := tests.New(t)
				browser := in(tc, tc.Owner)
				id := browser.PostJSON(patPath, testkit.JSON{"name": "CI", "abilities": []string{"*"}}).
					AssertCreated().String("data.id")
				tc.FailWrites(table, kinds...)

				browser.Delete(patPath + "/" + id).AssertStatus(500)
				if table == "personal_access_tokens" {
					browser.PatchJSON(patPath+"/"+id, testkit.JSON{"name": "Renamed"}).AssertStatus(500)
				}

				tc.Table("personal_access_tokens").AssertHas(testkit.Row{"name": "CI", "revoked_at": nil})
			})
		}
	})

	t.Run("answers a server error when tokens cannot be read", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser := in(tc, tc.Owner)
		id := browser.PostJSON(patPath, testkit.JSON{"name": "CI", "abilities": []string{"*"}}).
			AssertCreated().String("data.id")
		tc.BreakTable("personal_access_tokens")

		browser.GetJSON(patPath).AssertStatus(500)
		browser.GetJSON(patPath + "/" + id).AssertStatus(500)
		browser.PostJSON(patPath, testkit.JSON{"name": "CI", "abilities": []string{"*"}}).AssertStatus(500)
	})

	t.Run("leaves a token unwarned when the warning cannot be queued", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		in(tc, tc.Owner).PostJSON(patPath, testkit.JSON{
			"name": "Soon", "abilities": []string{"*"}, "expires_at": tests.Epoch().Add(48 * time.Hour),
		}).AssertCreated()
		tc.Drain()
		tc.FailWrites("river_job", "INSERT")

		job := &jobs.NotifyExpiringTokens{DB: tc.App.DB, Clock: tc.Clock, Mail: tc.App.Mail, Links: tc.App.Links}
		require.Error(t, job.Handle(t.Context(), nil))
		tc.Table("personal_access_tokens").AssertHas(testkit.Row{"name": "Soon", "expiry_notified_at": nil})

		tc.BreakTable("personal_access_tokens")
		require.Error(t, job.Handle(t.Context(), nil))
	})
}
