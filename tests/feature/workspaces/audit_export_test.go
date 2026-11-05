package workspaces_test

import (
	"bytes"
	"context"
	"encoding/csv"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/audit"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
download asks the client for the audit log as CSV, narrowed by the
query, and returns the answer.
*/
func download(c *testkit.Client, query string) *testkit.Response {
	return c.WithHeader("Accept", "text/csv").Send("GET", "/api/v1/audit-logs?"+query, nil)
}

/*
records reads an exported file into its rows, header first.
*/
func records(t *testing.T, res *testkit.Response) [][]string {
	t.Helper()

	rows, err := csv.NewReader(bytes.NewReader(res.Body)).ReadAll()
	require.NoError(t, err)

	return rows
}

/*
column returns the value of the named column in the row.
*/
func column(row []string, name string) string {
	for i, c := range audit.ExportColumns() {
		if c == name {
			return row[i]
		}
	}

	return ""
}

/*
rfc3339 writes the time as a query string value.
*/
func rfc3339(at time.Time) string {
	return url.QueryEscape(at.Format(time.RFC3339))
}

func TestAuditLogExport(t *testing.T) {
	t.Parallel()

	t.Run("an admin downloads a csv of a date range", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		admin, _ := join(t, tc, enums.RoleAdmin)
		start := tc.Clock.Now()
		tc.Clock.Travel(48 * time.Hour)
		inside := seed(t, tc, "team.created")
		tc.Clock.Travel(48 * time.Hour)
		seed(t, tc, "team.deleted")

		res := download(in(tc, admin), "filter[created_after]="+rfc3339(start.Add(time.Hour))+
			"&filter[created_before]="+rfc3339(start.Add(72*time.Hour))).AssertOK().
			AssertHeader("Content-Type", "text/csv; charset=utf-8").
			AssertHeader("Content-Disposition", `attachment; filename="dimmi-audit-`+
				tc.Workspace.Workspace.Slug+`-2026-11-02-2026-11-05.csv"`)

		rows := records(t, res)
		require.Len(t, rows, 2)
		assert.Equal(t, audit.ExportColumns(), rows[0])
		assert.Equal(t, auditID(inside), column(rows[1], "id"))
		assert.Equal(t, "2026-11-04T09:00:00Z", column(rows[1], "created_at"))
		assert.Equal(t, "member", column(rows[1], "actor_type"))
		assert.Equal(t, memberID(tc.Workspace.Owner), column(rows[1], "actor_id"))
		assert.Equal(t, "{}", column(rows[1], "changes"))
	})

	t.Run("covers the last 30 days when no range is asked for", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		tc.Clock.Travel(40 * 24 * time.Hour)
		recent := seed(t, tc, "team.created")

		res := download(in(tc, tc.Owner), "format=csv").AssertOK().
			AssertHeader("Content-Disposition", `attachment; filename="dimmi-audit-`+
				tc.Workspace.Workspace.Slug+`-2026-11-12-2026-12-12.csv"`)

		rows := records(t, res)
		require.Len(t, rows, 2)
		assert.Equal(t, auditID(recent), column(rows[1], "id"))
	})

	t.Run("a range over a year is refused", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		now := tc.Clock.Now()

		download(in(tc, tc.Owner), "filter[created_after]="+rfc3339(now.Add(-367*24*time.Hour))).
			AssertStatus(422).
			AssertJSONPath("code", "range_too_large").
			AssertJSONPath("max_days", 366)
		download(in(tc, tc.Owner), "filter[created_after]=yesterday").AssertStatus(400)
		download(in(tc, tc.Owner), "filter[nope]=1").AssertStatus(400)
		tc.Table("audit_logs").AssertCount(0, testkit.Row{"event": "audit_logs.exported"})
	})

	t.Run("csv cells cannot start a formula", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, err := factories.AuditLog(t.Context(), tc.App, audit.Entry{
			WorkspaceID: tc.Workspace.Workspace.ID, Event: "team.created", ActorID: tc.Workspace.Owner.ID,
			Subject: audit.Subject{Type: "team", Label: `=HYPERLINK("https://example.com")`},
		})
		require.NoError(t, err)

		rows := records(t, download(in(tc, tc.Owner), "filter[event]=team.created").AssertOK())

		require.Len(t, rows, 2)
		assert.Equal(t, `'=HYPERLINK("https://example.com")`, column(rows[1], "subject_label"))
	})

	t.Run("exporting leaves its own entry", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		seed(t, tc, "team.created")
		owner := in(tc, tc.Owner)

		download(owner, "filter[event]=team.created,workspace.created").AssertOK()

		owner.GetJSON("/api/v1/audit-logs?filter[event]=audit_logs.exported").AssertOK().
			AssertJSONPath("data.#", 1).
			AssertJSONPath("data.0.actor.id", memberID(tc.Workspace.Owner)).
			AssertJSONPath("data.0.subject.type", "workspace").
			AssertJSONPath("data.0.changes.rows", 2).
			AssertJSONPath("data.0.changes.completed", true).
			AssertJSONPath("data.0.changes.filters.event", []string{"team.created", "workspace.created"}).
			AssertJSONPath("data.0.changes.filters.created_before", "2026-11-02T09:00:00Z")
	})

	t.Run("a member cannot export", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		agent, _ := join(t, tc, enums.RoleMember)

		download(in(tc, agent), "").AssertForbidden().AssertJSONPath("code", "forbidden")
	})

	t.Run("a token with settings read can export", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, admin := join(t, tc, enums.RoleAdmin)

		rows := records(t, download(tc.ActingAsToken(admin, "settings:read"), "").AssertOK())
		assert.Equal(t, audit.ExportColumns(), rows[0])
		download(tc.ActingAsToken(admin, "members:read"), "").AssertForbidden().
			AssertJSONPath("code", "missing_ability")
	})

	t.Run("never exports another workspace's entries", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		other, err := factories.Workspace(t.Context(), tc.App, tc.CreateUser())
		require.NoError(t, err)
		foreign, err := factories.AuditLog(t.Context(), tc.App, audit.Entry{
			WorkspaceID: other.Workspace.ID, Event: "team.created", ActorID: other.Owner.ID,
		})
		require.NoError(t, err)

		rows := records(t, download(in(tc, tc.Owner), "").AssertOK())

		require.NotEmpty(t, rows[1:])
		for _, row := range rows[1:] {
			assert.NotEqual(t, auditID(foreign), column(row, "id"))
			assert.NotEqual(t, "team.created", column(row, "event"))
		}
	})

	t.Run("streams more entries than one read takes", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		exec(t, tc, `INSERT INTO audit_logs (workspace_id, event, actor_type, actor_label, created_at)
			SELECT $1, 'team.created', 'system', 'System', $2::timestamptz - make_interval(secs => n)
			FROM generate_series(1, 1001) AS n`, tc.Workspace.Workspace.ID, tc.Clock.Now())

		rows := records(t, download(in(tc, tc.Owner), "filter[event]=team.created").AssertOK())

		assert.Len(t, rows, 1002)
	})

	t.Run("a workspace exports ten times an hour", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)

		for range 10 {
			download(owner, "").AssertOK()
		}

		res := download(owner, "").AssertStatus(429).AssertJSONPath("code", "too_many_exports")
		assert.NotEmpty(t, res.String("retry_after"))
	})
}

func TestAuditLogExportFailures(t *testing.T) {
	t.Parallel()

	t.Run("a download the reader abandons is recorded as not completed", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		seed(t, tc, "team.created")
		export := tc.App.Actions.ExportAuditLogs
		ctx, cancel := context.WithCancel(t.Context())
		started, err := export.Start(ctx, audit.ExportAuditLogsInput{Membership: ownerMembership(tc)})
		require.NoError(t, err)
		cancel()

		var out strings.Builder
		require.NoError(t, export.Write(ctx, started, &out, resources.NewAuditLogRow))

		tc.Table("audit_logs").AssertHas(testkit.Row{"event": "audit_logs.exported"})
		in(tc, tc.Owner).GetJSON("/api/v1/audit-logs?filter[event]=audit_logs.exported").AssertOK().
			AssertJSONPath("data.0.changes.completed", false).
			AssertJSONPath("data.0.changes.rows", 0)
	})

	t.Run("a reader that refuses the file ends the stream", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		export := tc.App.Actions.ExportAuditLogs
		started, err := export.Start(t.Context(), audit.ExportAuditLogsInput{Membership: ownerMembership(tc)})
		require.NoError(t, err)

		require.NoError(t, export.Write(t.Context(), started, refusing{}, resources.NewAuditLogRow))

		in(tc, tc.Owner).GetJSON("/api/v1/audit-logs?filter[event]=audit_logs.exported").AssertOK().
			AssertJSONPath("data.0.changes.completed", false)
	})

	t.Run("a log that cannot be read ends the file after its header", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)
		tc.BreakTable("audit_logs")

		rows := records(t, download(owner, "").AssertOK())

		assert.Equal(t, [][]string{audit.ExportColumns()}, rows)
	})

	t.Run("answers busy when the export allowance cannot be counted", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)
		tc.BreakValkey()

		download(owner, "").AssertStatus(503)
	})
}

/*
refusing is a reader that takes nothing.
*/
type refusing struct{}

func (refusing) Write([]byte) (int, error) {
	return 0, context.Canceled
}
