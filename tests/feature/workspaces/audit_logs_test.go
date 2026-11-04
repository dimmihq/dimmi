package workspaces_test

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/audit"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
seed records an entry with the event in the test case's workspace, by
the owner, at the test clock's time.
*/
func seed(t *testing.T, tc *tests.TestCase, event string) models.AuditLog {
	t.Helper()

	entry, err := factories.AuditLog(t.Context(), tc.App, audit.Entry{
		WorkspaceID: tc.Workspace.Workspace.ID, Event: event, ActorID: tc.Workspace.Owner.ID,
	})
	require.NoError(t, err)

	return entry
}

/*
auditID returns the public id of the audit entry.
*/
func auditID(e models.AuditLog) string {
	return models.PublicID(models.PrefixAuditLog, e.ID)
}

func TestAuditLogRecording(t *testing.T) {
	t.Parallel()

	t.Run("records a role change with who did it, through what, from where", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, member := join(t, tc, enums.RoleMember)
		owner := in(tc, tc.Owner).WithHeader("User-Agent", "AuditTest/1.0")

		owner.PatchJSON("/api/v1/members/"+memberID(member), testkit.JSON{"role": "admin"}).AssertOK()

		res := owner.GetJSON("/api/v1/audit-logs?filter[event]=member.role_changed").AssertOK().
			AssertJSONPath("data.#", 1).
			AssertJSONPath("data.0.object", "audit_log").
			AssertJSONPath("data.0.actor.type", "member").
			AssertJSONPath("data.0.actor.id", memberID(tc.Workspace.Owner)).
			AssertJSONPath("data.0.actor.via.kind", "session").
			AssertJSONPath("data.0.subject.type", "member").
			AssertJSONPath("data.0.subject.id", memberID(member)).
			AssertJSONPath("data.0.changes.role.from", "member").
			AssertJSONPath("data.0.changes.role.to", "admin").
			AssertJSONPath("data.0.user_agent", "AuditTest/1.0")
		assert.Equal(t, tc.Owner.Name+" <"+tc.Owner.Email+">", res.String("data.0.actor.label"))
		assert.True(t, strings.HasPrefix(res.String("data.0.actor.via.id"), "ses_"))
		assert.NotEmpty(t, res.String("data.0.ip"))
		assert.NotEmpty(t, res.String("data.0.request_id"))
	})

	t.Run("records the workspace's creation and every change to its members", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		leaver, _ := join(t, tc, enums.RoleMember)
		_, admin := join(t, tc, enums.RoleAdmin)
		_, member := join(t, tc, enums.RoleMember)
		owner := in(tc, tc.Owner)

		owner.PatchJSON("/api/v1/workspace", testkit.JSON{"name": "Renamed"}).AssertOK()
		owner.PatchJSON("/api/v1/members/"+memberID(member), testkit.JSON{"status": "deactivated"}).AssertOK()
		owner.PatchJSON("/api/v1/members/"+memberID(member), testkit.JSON{"status": "active"}).AssertOK()
		in(tc, leaver).Delete("/api/v1/user/memberships/" + leaverMembership(t, tc, leaver)).AssertNoContent()
		owner.PatchJSON("/api/v1/members/"+memberID(admin), testkit.JSON{"role": "owner"}).AssertOK()

		res := in(tc, tc.Owner).GetJSON("/api/v1/audit-logs?sort=created_at&limit=100").AssertOK()
		assert.Equal(t, []string{
			"workspace.created", "member.joined", "workspace.updated", "member.deactivated", "member.reactivated",
			"member.left", "member.role_changed", "ownership.transferred", "member.role_changed",
		}, eventsOf(res))
		res.AssertJSONPath("data.2.changes.name.to", "Renamed").
			AssertJSONPath("data.2.subject.label", "Renamed")
	})

	t.Run("names the token a change was made through", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, member := join(t, tc, enums.RoleMember)
		tok, plain := tc.MintToken(tc.Workspace.Owner, "members:write")

		tc.Client().WithHeader("Authorization", "Bearer "+plain).
			PatchJSON("/api/v1/members/"+memberID(member), testkit.JSON{"role": "admin"}).AssertOK()

		res := in(tc, tc.Owner).GetJSON("/api/v1/audit-logs?filter[event]=member.role_changed").AssertOK().
			AssertJSONPath("data.0.actor.via.kind", "personal_token")
		assert.Contains(t, res.String("data.0.actor.label"), " via Token Test token (")
		via := models.PublicID(models.PrefixPersonalToken, tok.ID)
		assert.Equal(t, via, res.String("data.0.actor.via.id"))
		in(tc, tc.Owner).GetJSON("/api/v1/audit-logs?filter[via]="+via).AssertOK().AssertJSONPath("data.#", 1)
	})

	t.Run("undoes the change when its entry cannot be written", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, member := join(t, tc, enums.RoleMember)
		tc.BreakTable("audit_logs")

		in(tc, tc.Owner).PatchJSON("/api/v1/members/"+memberID(member), testkit.JSON{"role": "admin"}).
			AssertStatus(500)

		tc.Table("workspace_members").AssertHas(testkit.Row{"id": member.ID, "role": "member"})
	})

	t.Run("writes nothing when nothing changed", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, member := join(t, tc, enums.RoleMember)

		in(tc, tc.Owner).PatchJSON("/api/v1/members/"+memberID(member), testkit.JSON{"role": "member"}).AssertOK()

		tc.Table("audit_logs").AssertCount(0, testkit.Row{"event": "member.role_changed"})
	})

	t.Run("hides secrets in the changes however deep they sit", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		entry, err := factories.AuditLog(t.Context(), tc.App, audit.Entry{
			WorkspaceID: tc.Workspace.Workspace.ID, Event: "channel.connected",
			Changes: map[string]any{
				"client_secret": "s3cret", "name": "Support",
				"settings": map[string]any{"api_key": "k", "list": []any{map[string]any{"password": "p"}}},
			},
		})
		require.NoError(t, err)

		in(tc, tc.Owner).GetJSON("/api/v1/audit-logs/"+auditID(entry)).AssertOK().
			AssertJSONPath("data.actor.type", "system").
			AssertJSONPath("data.actor.label", "System").
			AssertJSONPath("data.changes.client_secret", audit.Redacted).
			AssertJSONPath("data.changes.name", "Support").
			AssertJSONPath("data.changes.settings.api_key", audit.Redacted).
			AssertJSONPath("data.changes.settings.list.0.password", audit.Redacted)
	})

	t.Run("never lets the application edit an entry", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		seed(t, tc, "workspace.updated")

		_, err := tc.App.DB.Exec(t.Context(), "UPDATE audit_logs SET event = 'changed.event'")

		require.ErrorContains(t, err, "permission denied")
	})
}

/*
eventsOf returns the event names of a page of entries, in order.
*/
func eventsOf(res *testkit.Response) []string {
	var page struct {
		Data []struct {
			Event string `json:"event"`
		} `json:"data"`
	}
	res.Decode(&page)

	names := make([]string, 0, len(page.Data))
	for _, entry := range page.Data {
		names = append(names, entry.Event)
	}

	return names
}

/*
leaverMembership returns the public id of the user's membership of the
test case's workspace.
*/
func leaverMembership(t *testing.T, tc *tests.TestCase, user models.User) string {
	t.Helper()

	m, err := models.New(tc.App.DB).FindWorkspaceMemberForUser(t.Context(), models.FindWorkspaceMemberForUserParams{
		WorkspaceID: tc.Workspace.Workspace.ID, UserID: user.ID,
	})
	require.NoError(t, err)

	return memberID(m)
}

func TestAuditLogIndex(t *testing.T) {
	t.Parallel()

	t.Run("lists newest first and pages with a cursor both ways", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		for range 3 {
			tc.Clock.Travel(time.Minute)
			seed(t, tc, "team.created")
		}

		owner := in(tc, tc.Owner)
		first := owner.GetJSON("/api/v1/audit-logs?limit=2").AssertOK().AssertJSONPath("data.#", 2)
		second := owner.GetJSON("/api/v1/audit-logs?limit=2&cursor=" + first.String("meta.next_cursor")).AssertOK()
		third := owner.GetJSON("/api/v1/audit-logs?limit=2&cursor="+second.String("meta.next_cursor")).AssertOK().
			AssertJSONPath("meta.next_cursor", nil)
		assert.Equal(t, "team.created", first.String("data.0.event"))
		assert.Equal(t, "workspace.created", third.String("data.0.event"))

		ascending := owner.GetJSON("/api/v1/audit-logs?sort=created_at&limit=4").AssertOK()
		owner.GetJSON("/api/v1/audit-logs?sort=created_at&cursor="+ascending.String("meta.next_cursor")).AssertOK().
			AssertJSONPath("data.#", 1).
			AssertJSONPath("data.0.id", first.String("data.0.id"))
	})

	t.Run("narrows by event, actor, subject and time", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, member := join(t, tc, enums.RoleMember)
		start := tc.Clock.Now()
		tc.Clock.Travel(time.Hour)
		in(tc, tc.Owner).PatchJSON("/api/v1/members/"+memberID(member), testkit.JSON{"role": "admin"}).AssertOK()
		tc.Clock.Travel(time.Hour)
		late := seed(t, tc, "team.created")
		owner := in(tc, tc.Owner)

		owner.GetJSON("/api/v1/audit-logs?filter[event]=team.created,member.role_changed").AssertOK().
			AssertJSONPath("data.#", 2)
		owner.GetJSON("/api/v1/audit-logs?filter[actor]="+memberID(tc.Workspace.Owner)).AssertOK().
			AssertJSONPath("data.#", 4)
		owner.GetJSON("/api/v1/audit-logs?filter[subject]="+memberID(member)).AssertOK().
			AssertJSONPath("data.#", 1)

		after := url.QueryEscape(start.Add(30 * time.Minute).Format(time.RFC3339))
		before := url.QueryEscape(late.CreatedAt.Format(time.RFC3339))
		owner.GetJSON("/api/v1/audit-logs?filter[created_after]="+after+"&filter[created_before]="+before).
			AssertOK().
			AssertJSONPath("data.#", 1).
			AssertJSONPath("data.0.event", "member.role_changed")
	})

	t.Run("refuses filters that name nothing and forged cursors", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)

		for _, query := range []string{
			"filter[actor]=nobody", "filter[created_after]=yesterday", "filter[via]=mem_x", "filter[kind]=x",
			"sort=event", "cursor=forged",
		} {
			owner.GetJSON("/api/v1/audit-logs?" + query).AssertStatus(400)
		}
	})
}

func TestAuditLogAccess(t *testing.T) {
	t.Parallel()

	t.Run("lets owners and admins read, and refuses members", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		admin, _ := join(t, tc, enums.RoleAdmin)
		agent, _ := join(t, tc, enums.RoleMember)
		entry := seed(t, tc, "team.created")

		in(tc, admin).GetJSON("/api/v1/audit-logs").AssertOK()
		in(tc, admin).GetJSON("/api/v1/audit-logs/" + auditID(entry)).AssertOK()
		in(tc, agent).GetJSON("/api/v1/audit-logs").AssertForbidden().AssertJSONPath("code", "forbidden")
		in(tc, agent).GetJSON("/api/v1/audit-logs/" + auditID(entry)).AssertForbidden()
	})

	t.Run("lets tokens read with settings:read, within their role", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, admin := join(t, tc, enums.RoleAdmin)
		_, agent := join(t, tc, enums.RoleMember)

		tc.ActingAsToken(admin, "settings:read").GetJSON("/api/v1/audit-logs").AssertOK()
		tc.ActingAsToken(admin, "members:read").GetJSON("/api/v1/audit-logs").AssertForbidden().
			AssertJSONPath("code", "missing_ability")
		tc.ActingAsToken(agent, "settings:read").GetJSON("/api/v1/audit-logs").AssertForbidden()
	})

	t.Run("never shows another workspace's entries", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		other, err := factories.Workspace(t.Context(), tc.App, tc.CreateUser())
		require.NoError(t, err)
		foreign, err := factories.AuditLog(t.Context(), tc.App, audit.Entry{
			WorkspaceID: other.Workspace.ID, Event: "team.created", ActorID: other.Owner.ID,
		})
		require.NoError(t, err)
		owner := in(tc, tc.Owner)

		owner.GetJSON("/api/v1/audit-logs/"+auditID(foreign)).AssertNotFound().AssertJSONPath("code", "not_found")
		owner.GetJSON("/api/v1/audit-logs/aud_nope").AssertNotFound()
		owner.GetJSON("/api/v1/audit-logs/" + models.PublicID(models.PrefixAuditLog, ids.New())).AssertNotFound()
		res := owner.GetJSON("/api/v1/audit-logs?limit=100").AssertOK()
		assert.NotContains(t, res.String("data.#.id"), auditID(foreign))
	})
}

func TestPruneAuditLogs(t *testing.T) {
	t.Parallel()

	t.Run("deletes entries older than the retention period and keeps the rest", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		old := seed(t, tc, "team.created")
		tc.Clock.Travel(400 * 24 * time.Hour)
		recent := seed(t, tc, "team.created")

		require.NoError(t, tc.App.Queue.Push(t.Context(), jobs.PruneAuditLogsArgs{}))

		keep := &jobs.PruneAuditLogs{DB: tc.App.DB, Clock: tc.Clock}
		require.NoError(t, keep.Handle(t.Context(), nil))
		tc.Table("audit_logs").AssertHas(testkit.Row{"id": old.ID})

		prune := &jobs.PruneAuditLogs{DB: tc.App.DB, Clock: tc.Clock, RetentionDays: 365}
		require.NoError(t, prune.Handle(t.Context(), nil))
		tc.Table("audit_logs").AssertMissing(testkit.Row{"id": old.ID}).AssertHas(testkit.Row{"id": recent.ID})
	})

	t.Run("reports a log it cannot reach", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		tc.BreakTable("audit_logs")

		prune := &jobs.PruneAuditLogs{DB: tc.App.DB, Clock: tc.Clock, RetentionDays: 365}
		require.Error(t, prune.Handle(t.Context(), nil))
	})
}

func TestAuditLogFailures(t *testing.T) {
	t.Parallel()

	t.Run("answers a server error when the log cannot be read", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		entry := seed(t, tc, "team.created")
		owner := in(tc, tc.Owner)
		tc.BreakTable("audit_logs")

		owner.GetJSON("/api/v1/audit-logs").AssertStatus(500)
		owner.GetJSON("/api/v1/audit-logs/" + auditID(entry)).AssertStatus(500)
	})

	t.Run("refuses an entry naming a member the workspace does not have", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		_, err := factories.AuditLog(t.Context(), tc.App, audit.Entry{
			WorkspaceID: tc.Workspace.Workspace.ID, Event: "team.created", ActorID: ids.New(),
		})

		require.Error(t, err)
		tc.Table("audit_logs").AssertMissing(testkit.Row{"event": "team.created"})
	})

	t.Run("fails an event about a workspace or member that does not exist", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		ws, owner := tc.Workspace.Workspace.ID, tc.Workspace.Owner

		for _, e := range []any{
			appevents.WorkspaceCreated{WorkspaceID: ids.New(), CreatedBy: owner.UserID},
			appevents.WorkspaceCreated{WorkspaceID: ws, CreatedBy: ids.New()},
			appevents.WorkspaceUpdated{WorkspaceID: ids.New(), UpdatedBy: owner.ID, Changes: []string{"name"}},
			appevents.MemberRoleChanged{WorkspaceID: ws, MemberID: ids.New(), ChangedBy: owner.ID},
		} {
			require.Error(t, tc.App.Events.Dispatch(t.Context(), nil, e))
		}
	})
}
