package migrations_test

import (
	"testing"

	"github.com/dimmihq/dimmi/tests"
)

func TestMigrationRateLimit(t *testing.T) {
	t.Parallel()

	t.Run("migrations count against a minute of their own", func(t *testing.T) {
		t.Parallel()
		d := open(t, tests.WithEnv(
			"DIMMI_API__LIMITS__MIGRATION_PER_MINUTE=3", "DIMMI_API__LIMITS__TOKEN_PER_MINUTE=60",
		))

		d.client.GetJSON(d.path("")).AssertOK().AssertHeader("X-RateLimit-Scope", "migration").
			AssertHeader("X-RateLimit-Limit", "3")
		d.client.GetJSON(d.path("")).AssertOK()
		d.client.GetJSON(d.path("")).AssertStatus(429).AssertHeader("X-RateLimit-Scope", "migration")

		d.TC.ActingAsToken(d.TC.Workspace.Owner, "members:read").GetJSON("/api/v1/members").AssertOK().
			AssertHeader("X-RateLimit-Scope", "token")
	})
}
