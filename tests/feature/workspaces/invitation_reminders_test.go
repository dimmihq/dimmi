package workspaces_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/jobs"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
sweepReminders runs the daily reminder sweep the given number of times.
*/
func sweepReminders(t *testing.T, tc *tests.TestCase, times int) {
	t.Helper()

	for range times {
		require.NoError(t, tc.App.Queue.Push(t.Context(), jobs.RemindInvitersArgs{}))
		tc.Drain()
	}
}

func TestRemindInviters(t *testing.T) {
	t.Parallel()

	t.Run("the inviter is reminded once after three days", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		invite(tc, "grace@example.com").AssertOK()

		tc.Clock.Travel(2 * 24 * time.Hour)
		sweepReminders(t, tc, 1)
		none, _ := tests.Sent[appmail.InvitationPendingMail](tc, fake)
		assert.Empty(t, none)

		tc.Clock.Travel(25 * time.Hour)
		sweepReminders(t, tc, 2)

		mails, messages := tests.Sent[appmail.InvitationPendingMail](tc, fake)
		require.Len(t, mails, 1)
		assert.Equal(t, "grace@example.com", mails[0].Email)
		assert.Equal(t, tc.Workspace.Workspace.Name, mails[0].Workspace)
		assert.Equal(t, tests.Epoch().UTC().Format("2 January 2006"), mails[0].SentOn)
		assert.Contains(t, mails[0].MembersURL, "/d/settings/members")
		assert.Equal(t, []string{tc.Owner.Email}, tests.SentTo(messages[0]))
	})

	t.Run("answered and expired invitations are not reminded about", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		invite(tc, "gone@example.com").AssertOK()
		in(tc, tc.Owner).Delete("/api/v1/invitations/" + invitationID(t, tc)).AssertNoContent()
		invite(tc, "late@example.com").AssertOK()

		tc.Clock.Travel(8 * 24 * time.Hour)
		sweepReminders(t, tc, 1)

		mails, _ := tests.Sent[appmail.InvitationPendingMail](tc, fake)
		assert.Empty(t, mails)
	})

	t.Run("an inviter who left is not reminded", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		admin, member := join(t, tc, enums.RoleAdmin)
		in(tc, admin).PostJSON("/api/v1/invitations", map[string]any{
			"invitations": []map[string]any{{"email": "grace@example.com", "role": "member"}},
		}).AssertOK()
		require.NoError(t, factories.Deactivate(t.Context(), tc.App, member))

		tc.Clock.Travel(4 * 24 * time.Hour)
		sweepReminders(t, tc, 1)

		mails, _ := tests.Sent[appmail.InvitationPendingMail](tc, fake)
		assert.Empty(t, mails)
	})
}
