package email_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/mail"

	"github.com/dimmihq/dimmi/app/enums"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/config"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
judged turns on the cloud's pause rates for the test case, whichever
edition the tests were built for.
*/
func judged(tc *tests.TestCase) {
	tc.App.EmailReputation.Rates = config.EmailReputation{BouncePauseRate: 0.05, ComplaintPauseRate: 0.001}
}

/*
bounceReply sends a reply to a new customer and has it bounce for good.
*/
func bounceReply(t *testing.T, tc *tests.TestCase) {
	t.Helper()

	address := defaultAddress(tc)
	customer := unique("gone")

	result, err := send(tc, memberReply(tc, startConversation(tc, address, customer), customer))
	require.NoError(t, err)

	deliver(tc, fixture(t, "bounce_dsn.eml", map[string]string{
		"TO": address.ForwardingAddress, "FORWARDING": address.ForwardingAddress, "RECIPIENT": customer,
		"ORIGINAL": result.ChannelMessageID,
	}), "")
	tc.Drain()
}

func TestSendingPause(t *testing.T) {
	t.Parallel()

	t.Run("counts what is sent and what bounces, and pauses bulk email past the rate", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		judged(tc)
		ws := tc.Workspace.Workspace.ID
		owner := tc.ActingAsOwner()
		outbox := tc.Mail()

		bounceReply(t, tc)

		tc.Table("email_sending_stats").AssertCount(1, map[string]any{"sent": 1, "hard_bounced": 1, "complained": 0})
		tc.Table("email_sending_profiles").AssertCount(0, map[string]any{"status": "paused"})
		owner.GetJSON("/api/v1/email-sending").AssertOK().
			AssertJSONPath("data.status", "active").
			AssertJSONPath("data.sent", 1).
			AssertJSONPath("data.hard_bounced", 1).
			AssertJSONPath("data.minimum_sent", 100).
			AssertJSONPath("data.window_days", 7)

		_, err := tc.Operator().Exec(t.Context(),
			`UPDATE email_sending_stats SET sent = 150, hard_bounced = 7 WHERE workspace_id = $1`, ws)
		require.NoError(t, err)
		_, err = tc.Operator().Exec(t.Context(), `INSERT INTO broadcasts (workspace_id, name, channel, status)
			VALUES ($1, 'Launch', 'email', 'sending'), ($1, 'Draft', 'email', 'draft')`, ws)
		require.NoError(t, err)

		bounceReply(t, tc)

		tc.Table("email_sending_profiles").AssertCount(1, map[string]any{
			"workspace_id": ws, "status": "paused", "paused_reason": "bounce_rate",
		})
		tc.Table("broadcasts").AssertCount(1, map[string]any{"status": "paused", "pause_reason": "sending_paused"})
		tc.Table("broadcasts").AssertCount(1, map[string]any{"status": "draft"})
		tc.Table("audit_logs").AssertCount(1, map[string]any{"event": "email_sending.paused"})
		mail.AssertSent(t, outbox, func(m appmail.EmailSendingPausedMail) bool {
			return m.Reason == "bounce_rate" && m.Sent == 151 && m.HardBounced == 8
		})
		owner.GetJSON("/api/v1/email-sending").AssertOK().
			AssertJSONPath("data.status", "paused").
			AssertJSONPath("data.paused_reason", "bounce_rate")

		bounceReply(t, tc)
		tc.Table("audit_logs").AssertCount(1, map[string]any{"event": "email_sending.paused"})
	})

	t.Run("keeps replies going while paused, and only an owner resumes", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		judged(tc)
		ws := tc.Workspace.Workspace.ID
		bounceReply(t, tc)
		_, err := tc.Operator().Exec(t.Context(),
			`UPDATE email_sending_stats SET sent = 150, hard_bounced = 20 WHERE workspace_id = $1`, ws)
		require.NoError(t, err)
		paused, err := tc.App.EmailReputation.Judge(t.Context(), ws)
		require.NoError(t, err)
		require.True(t, paused)

		address := defaultAddress(tc)
		customer := unique("fine")
		_, err = send(tc, memberReply(tc, startConversation(tc, address, customer), customer))
		require.NoError(t, err)

		user := tc.CreateUser()
		_, err = factories.Member(t.Context(), tc.App, factories.Membership{
			Workspace: tc.Workspace.Workspace, User: user, Role: enums.RoleAdmin,
		})
		require.NoError(t, err)
		tc.ActingAs(user).PostJSON("/api/v1/email-sending/resumptions", nil).AssertForbidden()
		tc.Table("email_sending_profiles").AssertCount(1, map[string]any{"status": "paused"})

		owner := tc.ActingAsOwner()
		owner.PostJSON("/api/v1/email-sending/resumptions", nil).AssertCreated().
			AssertJSONPath("data.status", "active").
			AssertJSONPath("data.sent", 0)
		tc.Table("email_sending_profiles").AssertCount(1, map[string]any{"status": "active", "paused_reason": nil})
		tc.Table("email_sending_stats").AssertCount(0)
		tc.Table("audit_logs").AssertCount(1, map[string]any{"event": "email_sending.resumed"})

		owner.PostJSON("/api/v1/email-sending/resumptions", nil).AssertCreated().
			AssertJSONPath("data.status", "active")
		tc.Table("audit_logs").AssertCount(1, map[string]any{"event": "email_sending.resumed"})
	})

	t.Run("never pauses with too few emails or with the rates off", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		ws := tc.Workspace.Workspace.ID
		judged(tc)
		outbox := tc.Mail()
		bounceReply(t, tc)

		for _, sent := range outbox.Sent() {
			_, told := sent.Mailable.(appmail.EmailSendingPausedMail)
			assert.False(t, told)
		}
		tc.Table("email_sending_profiles").AssertCount(0, map[string]any{"status": "paused"})

		tc.App.EmailReputation.Rates = config.EmailReputation{}
		_, err := tc.Operator().Exec(t.Context(),
			`UPDATE email_sending_stats SET sent = 150, hard_bounced = 150 WHERE workspace_id = $1`, ws)
		require.NoError(t, err)

		paused, err := tc.App.EmailReputation.Judge(t.Context(), ws)
		require.NoError(t, err)
		assert.False(t, paused)
	})
}
