package notifications_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/notifications"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
delivery is one line of the delivery log as the API returns it.
*/
type delivery struct {
	ID         string  `json:"id"`
	DeliveryID string  `json:"delivery_id"`
	MemberID   string  `json:"member_id"`
	Trigger    string  `json:"trigger"`
	Channel    string  `json:"channel"`
	Status     string  `json:"status"`
	Reason     *string `json:"reason"`
	Title      string  `json:"title"`
}

func deliveries(t *testing.T, c *testkit.Client, query string) []delivery {
	t.Helper()
	var body struct {
		Data []delivery `json:"data"`
	}
	c.GetJSON("/api/v1/notification-deliveries" + query).AssertOK().Decode(&body)
	return body.Data
}

/*
line finds the log line of the trigger on the channel.
*/
func line(lines []delivery, trigger, channel string) (delivery, bool) {
	for _, l := range lines {
		if l.Trigger == trigger && l.Channel == channel {
			return l, true
		}
	}
	return delivery{}, false
}

func reasonOf(l delivery) string {
	if l.Reason == nil {
		return ""
	}
	return *l.Reason
}

/*
quietNow turns the owner's quiet hours on around the test clock's now.
*/
func (d *desk) quietNow() {
	d.t.Helper()
	now := d.TC.Clock.Now().UTC()
	d.Owner.PatchJSON("/api/v1/notification-settings", testkit.JSON{
		"timezone": "UTC", "quiet_hours_enabled": true,
		"quiet_hours_start": now.Add(-time.Hour).Format("15:04"), "quiet_hours_end": now.Add(time.Hour).Format("15:04"),
	}).AssertOK()
}

func TestDeliveryLog(t *testing.T) {
	t.Parallel()

	t.Run("records the bell and the email as sent under one delivery id", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		d.visitor().start("My invoice is wrong")
		d.TC.Drain()
		require.Len(t, d.alerts(d.TC.Owner.Email), 1)

		lines := deliveries(t, d.Owner, "")
		inApp, ok := line(lines, "new_conversation", "in_app")
		require.True(t, ok)
		email, ok := line(lines, "new_conversation", "email")
		require.True(t, ok)
		assert.Equal(t, "sent", inApp.Status)
		assert.Equal(t, "sent", email.Status)
		assert.Equal(t, inApp.DeliveryID, email.DeliveryID)
		assert.Contains(t, email.Title, "started a conversation")

		row := bell(t, d.Owner).String("data.0.id")
		var of struct {
			Data []delivery `json:"data"`
		}
		d.Owner.GetJSON("/api/v1/notifications/" + row + "/deliveries").AssertOK().Decode(&of)
		assert.Len(t, of.Data, 2)
	})

	t.Run("records a channel turned off as skipped with the reason", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		d.Owner.PatchJSON("/api/v1/notification-preferences", testkit.JSON{"preferences": []testkit.JSON{
			{"trigger": "new_conversation", "channel": "email", "enabled": false},
		}}).AssertOK()
		d.visitor().start("Hello")
		d.TC.DrainLater()

		email, ok := line(deliveries(t, d.Owner, "?filter[status]=skipped"), "new_conversation", "email")
		require.True(t, ok)
		assert.Equal(t, "skipped", email.Status)
		assert.Equal(t, notifications.ReasonPreferenceOff, reasonOf(email))
	})

	t.Run("records quiet hours and sends one summary once they end", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		d.quietNow()
		d.visitor().start("First")
		d.visitor().start("Second")
		assert.Empty(t, d.alerts(d.TC.Owner.Email))

		held := deliveries(t, d.Owner, "?filter[channel]=email")
		require.Len(t, held, 2)
		for _, l := range held {
			assert.Equal(t, notifications.ReasonQuietHours, reasonOf(l))
		}

		runSummaries(t, d)
		summaries, _ := tests.Sent[notifications.QuietSummaryMail](d.TC, d.mail)
		assert.Empty(t, summaries, "no summary while quiet hours last")

		d.TC.Clock.Travel(3 * time.Hour)
		runSummaries(t, d)
		runSummaries(t, d)
		summaries, messages := tests.Sent[notifications.QuietSummaryMail](d.TC, d.mail)
		require.Len(t, summaries, 1)
		assert.Len(t, summaries[0].Lines, 2)
		assert.Contains(t, messages[0].Message.Headers["List-Unsubscribe"], "/n/u/")
	})

	t.Run("lets only someone who manages members read another member's log", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		bob, bobMember := d.Join(t, enums.RoleMember)
		admin, _ := d.Join(t, enums.RoleAdmin)
		d.visitor().start("Hello")
		d.TC.Drain()

		owner := models.PublicID(models.PrefixMember, d.TC.Workspace.Owner.ID)
		bob.GetJSON("/api/v1/notification-deliveries?filter[member]=" + owner).AssertForbidden()
		lines := deliveries(t, admin, "?filter[member]="+owner)
		require.NotEmpty(t, lines)
		for _, l := range lines {
			assert.Equal(t, owner, l.MemberID)
		}
		for _, l := range deliveries(t, bob, "") {
			assert.Equal(t, models.PublicID(models.PrefixMember, bobMember.ID), l.MemberID)
		}
	})
}

func runSummaries(t *testing.T, d *desk) {
	t.Helper()
	require.NoError(t, d.TC.App.Queue.Push(t.Context(), jobs.SendQuietHoursSummariesArgs{}))
	d.TC.Drain()
}

func TestUnsubscribe(t *testing.T) {
	t.Parallel()

	key := []byte("0123456789abcdef0123456789abcdef")
	now := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)
	ref := notifications.UnsubscribeRef{Trigger: "mentioned", Expires: now.Add(time.Hour)}

	t.Run("reads back a link it signed", func(t *testing.T) {
		t.Parallel()
		got, err := notifications.ReadUnsubscribe(key, notifications.SignUnsubscribe(key, ref), now)
		require.NoError(t, err)
		assert.Equal(t, "mentioned", got.Trigger)
	})

	t.Run("refuses a link signed with another key or changed", func(t *testing.T) {
		t.Parallel()
		token := notifications.SignUnsubscribe([]byte("another key of thirty-two bytes!"), ref)
		_, err := notifications.ReadUnsubscribe(key, token, now)
		require.ErrorIs(t, err, notifications.ErrBadUnsubscribe)

		payload, sig, _ := strings.Cut(notifications.SignUnsubscribe(key, ref), ".")
		_, err = notifications.ReadUnsubscribe(key, payload+"A."+sig, now)
		require.ErrorIs(t, err, notifications.ErrBadUnsubscribe)
	})

	t.Run("refuses a link that expired", func(t *testing.T) {
		t.Parallel()
		_, err := notifications.ReadUnsubscribe(key, notifications.SignUnsubscribe(key, ref), now.Add(2*time.Hour))
		require.ErrorIs(t, err, notifications.ErrBadUnsubscribe)
	})

	t.Run("puts the one-click headers on alert emails and turns that email off by POST", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		d.visitor().start("Hello")
		d.TC.DrainLater()
		_, messages := tests.Sent[notifications.MemberAlertMail](d.TC, d.mail)
		require.Len(t, messages, 1)
		headers := messages[0].Message.Headers
		assert.Equal(t, "List-Unsubscribe=One-Click", headers["List-Unsubscribe-Post"])
		link := strings.Trim(headers["List-Unsubscribe"], "<>")
		require.Contains(t, link, "/n/u/")
		token := link[strings.LastIndex(link, "/")+1:]

		oneClick(d, token).AssertOK()
		enabled := func() bool {
			var body struct {
				Data struct {
					Cells []struct {
						Trigger string `json:"trigger"`
						Channel string `json:"channel"`
						Enabled bool   `json:"enabled"`
					} `json:"cells"`
				} `json:"data"`
			}
			d.Owner.GetJSON("/api/v1/notification-preferences").AssertOK().Decode(&body)
			for _, c := range body.Data.Cells {
				if c.Trigger == "new_conversation" && c.Channel == "email" {
					return c.Enabled
				}
			}
			return false
		}
		assert.False(t, enabled())

		guest := d.TC.Client()
		guest.GetJSON("/api/v1/notification-unsubscribes/"+token).AssertOK().
			AssertJSONPath("data.subscribed", false).AssertJSONPath("data.trigger", "new_conversation")
		guest.Delete("/api/v1/notification-unsubscribes/"+token).AssertOK().
			AssertJSONPath("data.subscribed", true)
		assert.True(t, enabled())

		guest.GetJSON("/api/v1/notification-unsubscribes/" + token + "x").AssertNotFound()
		oneClick(d, token+"x").AssertNotFound()
	})
}

/*
oneClick posts to the link as a mail app's unsubscribe button does.
*/
func oneClick(d *desk, token string) *testkit.Response {
	return d.TC.Client().Send("POST", "/n/u/"+token, []byte("List-Unsubscribe=One-Click"))
}
