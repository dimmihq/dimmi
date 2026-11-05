package workspaces_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/listeners"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
attempt tries to sign in with the email and password.
*/
func attempt(tc *tests.TestCase, email, password string) *testkit.Response {
	return tc.Client().WithHeader("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Firefox/130.0").
		PostJSON("/api/v1/sessions", testkit.JSON{"grant": "password", "email": email, "password": password})
}

/*
signInEntries returns the owner's view of the audit log narrowed to the
event about the member.
*/
func signInEntries(tc *tests.TestCase, event string, member models.WorkspaceMember) *testkit.Response {
	return in(tc, tc.Owner).GetJSON("/api/v1/audit-logs?filter[event]=" + event + "&filter[subject]=" +
		memberID(member)).AssertOK()
}

func TestSignInAudit(t *testing.T) {
	t.Parallel()

	t.Run("a failed sign-in shows in each workspace of that person", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, member := join(t, tc, enums.RoleMember)
		own, err := factories.Workspace(t.Context(), tc.App, user)
		require.NoError(t, err)

		attempt(tc, user.Email, "not-the-password").AssertStatus(422)
		tc.Drain()

		res := signInEntries(tc, "member.sign_in_failed", member).
			AssertJSONPath("data.#", 1).
			AssertJSONPath("data.0.actor.type", "system").
			AssertJSONPath("data.0.actor.label", "Unknown").
			AssertJSONPath("data.0.subject.label", user.Name).
			AssertJSONPath("data.0.changes.reason", "invalid_credentials").
			AssertJSONPath("data.0.changes.method", "password")
		assert.NotEmpty(t, res.String("data.0.changes.device_label"))
		assert.NotEmpty(t, res.String("data.0.ip"))
		assert.Equal(t, res.String("data.0.ip"), res.String("data.0.changes.ip"))
		assert.Contains(t, res.String("data.0.user_agent"), "Firefox")
		tc.Table("audit_logs").AssertHas(testkit.Row{
			"workspace_id": own.Workspace.ID, "event": "member.sign_in_failed", "subject_id": own.Owner.ID,
		})
	})

	t.Run("an unknown address leaves no entry", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		attempt(tc, "nobody@example.com", "not-the-password").AssertStatus(422)
		tc.Drain()

		tc.Table("audit_logs").AssertCount(0, testkit.Row{"event": "member.sign_in_failed"})
	})

	t.Run("a colleague's second factor removal shows to admins", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, member := join(t, tc, enums.RoleMember)
		browser := in(tc, user)
		account, err := factories.WithTwoFactor(t.Context(), tc.App, user)
		require.NoError(t, err)

		browser.Delete("/api/v1/user/auth-factors/" + models.PublicID(models.PrefixAuthFactor, account.TOTP.ID)).
			AssertNoContent()
		tc.Drain()

		signInEntries(tc, "member.auth_factor_removed", member).
			AssertJSONPath("data.#", 1).
			AssertJSONPath("data.0.actor.type", "member").
			AssertJSONPath("data.0.actor.id", memberID(member)).
			AssertJSONPath("data.0.changes.factor_type", "totp")
	})

	t.Run("a code or password never lands in changes", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, _ := join(t, tc, enums.RoleMember)
		browser := in(tc, user)
		account, err := factories.WithTwoFactor(t.Context(), tc.App, user)
		require.NoError(t, err)
		phrase := "a brand new passphrase for audit"

		attempt(tc, user.Email, "wrong passphrase guess").AssertStatus(422)
		browser.PatchJSON("/api/v1/user/password", testkit.JSON{
			"current_password": factories.Password, "password": phrase,
		}).AssertOK()
		tc.Drain()

		var events, changes []string
		require.NoError(t, tc.App.DB.QueryRow(t.Context(),
			"SELECT array_agg(event), array_agg(changes::text) FROM audit_logs WHERE subject_type = 'member'",
		).Scan(&events, &changes))
		joined := strings.Join(changes, " ")
		assert.Subset(t, events, []string{"member.sign_in_failed", "member.signed_in", "member.password_changed"})
		for _, secret := range append([]string{factories.Password, phrase, "wrong passphrase guess", "code"},
			account.RecoveryCodes...) {
			assert.NotContains(t, joined, secret)
		}
	})

	t.Run("mirror writes after login returns", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, member := join(t, tc, enums.RoleMember)

		attempt(tc, user.Email, factories.Password).AssertCreated()

		tc.Table("audit_logs").AssertCount(0, testkit.Row{"event": "member.signed_in", "subject_id": member.ID})
		tc.Drain()
		signInEntries(tc, "member.signed_in", member).
			AssertJSONPath("data.#", 1).
			AssertJSONPath("data.0.actor.id", memberID(member)).
			AssertJSONPath("data.0.actor.via", nil).
			AssertJSONPath("data.0.changes.methods", []string{"password"}).
			AssertJSONPath("data.0.created_at", "2026-11-02T09:00:00Z")
	})

	t.Run("a former member's entries stay", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, member := join(t, tc, enums.RoleMember)
		attempt(tc, user.Email, factories.Password).AssertCreated()
		tc.Drain()

		in(tc, tc.Owner).PatchJSON("/api/v1/members/"+memberID(member), testkit.JSON{"status": "deactivated"}).
			AssertOK()
		tc.Clock.Travel(time.Hour)
		attempt(tc, user.Email, factories.Password).AssertCreated()
		tc.Drain()

		signInEntries(tc, "member.signed_in", member).AssertJSONPath("data.#", 1)
	})

	t.Run("a workspace joined later gets no copy of earlier sign-ins", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user := tc.CreateUser()
		attempt(tc, user.Email, factories.Password).AssertCreated()
		tc.Clock.Travel(time.Minute)
		member := joinUser(t, tc, user)
		tc.Drain()

		tc.Table("audit_logs").AssertCount(0, testkit.Row{"event": "member.signed_in", "subject_id": member.ID})
	})

	t.Run("a change made at the command line names the system acting there", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, member := join(t, tc, enums.RoleMember)
		_, err := factories.WithTwoFactor(t.Context(), tc.App, user)
		require.NoError(t, err)

		code, out := tc.Command("user:disable-2fa", user.Email)
		require.Equal(t, 0, code, out)
		tc.Drain()

		signInEntries(tc, "member.two_factor_disabled", member).
			AssertJSONPath("data.#", 1).
			AssertJSONPath("data.0.actor.type", "system").
			AssertJSONPath("data.0.actor.via.kind", "cli")
	})

	t.Run("a blocked sign-in names the system, and other entries stay private", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, member := join(t, tc, enums.RoleMember)

		for _, kind := range []string{"login.locked", "logout.succeeded"} {
			require.NoError(t, tc.App.Events.Dispatch(t.Context(), nil, appevents.SecurityEventRecorded{
				UserID: user.ID, Type: kind, At: tc.Clock.Now(),
			}))
		}
		tc.Drain()

		signInEntries(tc, "member.sign_in_locked", member).
			AssertJSONPath("data.#", 1).
			AssertJSONPath("data.0.actor.type", "system").
			AssertJSONPath("data.0.actor.label", "System").
			AssertJSONPath("data.0.changes.reason", "too_many_attempts")
		tc.Table("audit_logs").AssertCount(0, testkit.Row{"subject_id": member.ID, "event": "logout.succeeded"})
	})
}

func TestSignInAuditFailures(t *testing.T) {
	t.Parallel()

	t.Run("retries when the audit log cannot be written", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, _ := join(t, tc, enums.RoleMember)
		mirror := &listeners.MirrorSecurityEventToAudit{DB: tc.App.DB, Record: tc.App.Actions.RecordAuditLog}
		job := &queue.Job[listeners.MirrorSecurityEventToAuditArgs]{Args: listeners.MirrorSecurityEventToAuditArgs{
			UserID: user.ID, Type: "login.succeeded", At: tc.Clock.Now(),
		}}
		tc.BreakTable("audit_logs")

		require.Error(t, mirror.Handle(t.Context(), job))
	})

	t.Run("retries when the memberships cannot be read", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		mirror := &listeners.MirrorSecurityEventToAudit{DB: tc.App.DB, Record: tc.App.Actions.RecordAuditLog}
		job := &queue.Job[listeners.MirrorSecurityEventToAuditArgs]{Args: listeners.MirrorSecurityEventToAuditArgs{
			UserID: ids.New(), Type: "login.succeeded", At: tc.Clock.Now(),
		}}
		tc.BreakTable("workspace_members")

		require.Error(t, mirror.Handle(t.Context(), job))
	})
}

/*
joinUser adds the existing user to the test case's workspace as a
member.
*/
func joinUser(t *testing.T, tc *tests.TestCase, user models.User) models.WorkspaceMember {
	t.Helper()

	member, err := factories.Member(t.Context(), tc.App, factories.Membership{
		Workspace: tc.Workspace.Workspace, User: user, Role: enums.RoleMember,
	})
	require.NoError(t, err)

	return member
}
