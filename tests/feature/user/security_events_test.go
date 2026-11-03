package user_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/tests"
)

func TestSecurityEventIndex(t *testing.T) {
	t.Parallel()

	t.Run("lists the caller's own log newest first, without device ids", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		tc.ActingAs(tc.CreateUser())
		tc.Clock.Travel(time.Minute)
		client := tc.ActingAsOwner()

		res := client.GetJSON("/api/v1/user/security-events").AssertOK().
			AssertJSONPath("data.#", 1).
			AssertJSONPath("data.0.object", "security_event").
			AssertJSONPath("data.0.type", "login.succeeded").
			AssertJSONPath("data.0.metadata.auth_methods", []string{"password"}).
			AssertJSONMissing("data.0.metadata.device_id").
			AssertJSONPath("meta.next_cursor", nil)
		assert.NotEmpty(t, res.String("data.0.ip"))
	})

	t.Run("pages with a cursor", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		for range 2 {
			tc.ActingAsOwner()
			tc.Clock.Travel(time.Minute)
		}

		client := tc.ActingAsOwner()

		first := client.GetJSON("/api/v1/user/security-events?limit=2").AssertOK().AssertJSONPath("data.#", 2)
		last := client.GetJSON("/api/v1/user/security-events?limit=2&cursor="+first.String("meta.next_cursor")).
			AssertOK().
			AssertJSONPath("data.#", 1).
			AssertJSONPath("meta.next_cursor", nil)
		assert.NotEqual(t, first.String("data.1.id"), last.String("data.0.id"))
		client.GetJSON("/api/v1/user/security-events?cursor=forged").AssertStatus(400)
	})

	t.Run("refuses access tokens and callers without a session", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.ActingAsToken(tc.Workspace.Owner, "*").GetJSON("/api/v1/user/security-events").
			AssertForbidden().
			AssertJSONPath("code", "first_party_only")
		tc.Client().GetJSON("/api/v1/user/security-events").AssertUnauthorized()
	})
}

func TestPruneSecurityEvents(t *testing.T) {
	t.Parallel()

	t.Run("deletes entries older than the retention period and keeps the rest", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		tc.ActingAsOwner()
		tc.Clock.Travel(400 * 24 * time.Hour)
		tc.ActingAsOwner()

		keep := &jobs.PruneSecurityEvents{DB: tc.App.DB, Clock: tc.Clock}
		require.NoError(t, keep.Handle(t.Context(), nil))
		tc.Table("security_events").AssertCount(2, testkit.Row{"user_id": tc.Owner.ID})

		prune := &jobs.PruneSecurityEvents{DB: tc.App.DB, Clock: tc.Clock, RetentionDays: 365}
		require.NoError(t, prune.Handle(t.Context(), nil))
		tc.Table("security_events").AssertCount(1, testkit.Row{"user_id": tc.Owner.ID})
	})
}
