package members_test

import (
	"testing"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/tests"
)

func TestMemberImportFailures(t *testing.T) {
	t.Parallel()

	t.Run("refuses to import while the counters are out of reach", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)
		tc.BreakValkey()

		upload(owner, dryRun, "email").AssertStatus(fhttp.StatusServiceUnavailable)
	})

	t.Run("answers a server error when the workspace cannot be read", func(t *testing.T) {
		t.Parallel()

		for _, table := range []string{"teams", "users"} {
			t.Run(table, func(t *testing.T) {
				t.Parallel()
				tc := tests.New(t)
				owner := in(tc, tc.Owner)
				tc.BreakTable(table)

				upload(owner, dryRun, "email", "a@example.com").AssertStatus(fhttp.StatusInternalServerError)
			})
		}
	})

	t.Run("answers a server error when a change cannot be written", func(t *testing.T) {
		t.Parallel()

		for name, failure := range map[string]struct {
			table, kind, line string
		}{
			"role":   {"workspace_members", "UPDATE", ",admin,"},
			"join":   {"team_members", "INSERT", ",,Billing"},
			"leave":  {"team_members", "DELETE", ",,Billing"},
			"invite": {"invitations", "INSERT", ""},
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				tc := tests.New(t)
				user, member := join(t, tc, enums.RoleMember)
				team(t, tc, "Support", member)
				team(t, tc, "Billing")
				owner := in(tc, tc.Owner)
				tc.FailWrites(failure.table, failure.kind)

				email := user.Email
				if name == "invite" {
					email = "new@example.com"
				}

				upload(owner, apply, "email,role,teams", email+failure.line).
					AssertStatus(fhttp.StatusInternalServerError)
			})
		}
	})
}

func TestMemberExportFailures(t *testing.T) {
	t.Parallel()

	t.Run("records the export even when the list breaks off", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner).WithHeader("Accept", "text/csv")
		tc.BreakTable("team_members")

		owner.Send("GET", "/api/v1/members", nil).AssertOK().AssertHeader("Content-Type", "text/csv; charset=utf-8")
		tc.Table("audit_logs").AssertHas(testkit.Row{"event": "members.exported"})
	})

	t.Run("still sends the file when its record cannot be written", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner).WithHeader("Accept", "text/csv")
		tc.FailWrites("audit_logs", "INSERT")

		owner.Send("GET", "/api/v1/members", nil).AssertOK()
	})
}
