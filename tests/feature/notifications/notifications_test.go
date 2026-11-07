package notifications_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/notifications"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
desk is a workspace with its owner, the owner's mail inbox and the
messenger customers write through.
*/
type desk struct {
	tests.Desk
	t    *testing.T
	mail *mail.Fake
	app  string
}

func newDesk(t *testing.T) *desk {
	t.Helper()
	d := &desk{Desk: tests.NewDesk(t), t: t}
	d.mail = d.TC.Mail()
	d.TC.InWorkspace(func(ctx context.Context) error {
		rows, err := models.New(d.TC.App.DB).ListWidgetSettings(ctx, d.TC.Workspace.Workspace.ID)
		require.NoError(t, err)
		require.NotEmpty(t, rows)
		d.app = models.PublicID(models.PrefixWidget, rows[0].ID)
		return nil
	})
	return d
}

/*
visitor is a customer writing through the messenger.
*/
type visitor struct {
	d     *desk
	token string
}

func (d *desk) visitor() visitor {
	d.t.Helper()
	res := d.TC.Client().PostJSON("/widget/v1/sessions", testkit.JSON{"app_id": d.app}).AssertCreated()
	return visitor{d: d, token: res.String("data.token")}
}

func (v visitor) client() *testkit.Client {
	return v.d.TC.Client().WithHeader("Authorization", "Bearer "+v.token)
}

/*
start has the customer write their first message and returns the
conversation's public id.
*/
func (v visitor) start(body string) string {
	v.d.t.Helper()
	return v.client().PostJSON("/widget/v1/conversations", testkit.JSON{
		"client_id": ids.New().String(), "body": body,
	}).AssertCreated().String("data.id")
}

/*
write has the customer send another message in the conversation.
*/
func (v visitor) write(conv, body string) {
	v.d.t.Helper()
	v.client().PostJSON("/widget/v1/conversations/"+conv+"/messages", testkit.JSON{
		"client_id": ids.New().String(), "body": body,
	}).AssertCreated()
}

/*
bell returns a member's notifications.
*/
func bell(t *testing.T, c *testkit.Client) *testkit.Response {
	t.Helper()
	return c.GetJSON("/api/v1/notifications").AssertOK()
}

/*
alerts returns the alert mails sent to the address.
*/
func (d *desk) alerts(to string) []notifications.MemberAlertMail {
	d.t.Helper()
	d.TC.DrainLater()
	sent, messages := tests.Sent[notifications.MemberAlertMail](d.TC, d.mail)
	var out []notifications.MemberAlertMail
	for i, m := range sent {
		for _, addr := range messages[i].Message.To {
			if addr.Email == to {
				out = append(out, m)
			}
		}
	}
	return out
}

func TestNotifications(t *testing.T) {
	t.Parallel()

	t.Run("tells the owner about a new conversation in the bell and by email", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		d.visitor().start("My invoice is wrong")
		d.TC.Drain()

		res := bell(t, d.Owner).AssertJSONPath("meta.unread_count", float64(1)).
			AssertJSONPath("meta.unseen_count", float64(1)).AssertJSONPath("data.0.trigger", "new_conversation").
			AssertJSONPath("data.0.body", "My invoice is wrong")
		assert.Contains(t, res.String("data.0.title"), "started a conversation")
		assert.Contains(t, res.String("data.0.link"), "/inbox/all/conv_")
		alerts := d.alerts(d.TC.Owner.Email)
		require.Len(t, alerts, 1)
		assert.Contains(t, alerts[0].Title, "started a conversation")
	})

	t.Run("folds a customer's replies into one row and skips email once the conversation is read", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		v := d.visitor()
		conv := v.start("Hello")
		d.TC.Drain()
		d.Owner.PatchJSON("/api/v1/conversations/"+conv+"/read-state", testkit.JSON{}).AssertOK()
		v.write(conv, "Are you there?")
		v.write(conv, "Still waiting")
		d.Owner.PatchJSON("/api/v1/conversations/"+conv+"/read-state", testkit.JSON{}).AssertOK()
		d.TC.Drain()

		res := bell(t, d.Owner)
		replies := 0
		for i := range 5 {
			if res.String("data."+itoa(i)+".trigger") == "customer_replied" {
				replies++
				assert.Equal(t, "2", res.String("data."+itoa(i)+".count"))
			}
		}
		assert.Equal(t, 1, replies)
		for _, a := range d.alerts(d.TC.Owner.Email) {
			assert.NotContains(t, a.Title, "replied")
		}
	})

	t.Run("tells only the member a conversation is given to, never the one who gave it", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		bob, bobMember := d.Join(t, enums.RoleMember)
		conv := d.visitor().start("Need help")
		d.TC.Drain()

		d.Owner.PatchJSON("/api/v1/conversations/"+conv, testkit.JSON{
			"assignee_id": models.PublicID(models.PrefixMember, bobMember.ID),
		}).AssertOK()
		d.TC.Drain()

		bell(t, bob).AssertJSONPath("data.0.trigger", "conversation_assigned")
		owner := bell(t, d.Owner)
		assert.NotEqual(t, "conversation_assigned", owner.String("data.0.trigger"))
	})

	t.Run("tells a mentioned teammate and nobody else", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		bob, bobMember := d.Join(t, enums.RoleMember)
		conv := d.visitor().start("Question")
		d.TC.Drain()
		d.Owner.PostJSON("/api/v1/conversations/"+conv+"/notes", testkit.JSON{
			"body": "<@" + models.PublicID(models.PrefixMember, bobMember.ID) + "> can you take this?",
		}).AssertCreated()
		d.TC.Drain()

		res := bell(t, bob).AssertJSONPath("data.0.trigger", "mentioned")
		assert.Contains(t, res.String("data.0.title"), "mentioned you")
		assert.Empty(t, filter(bell(t, d.Owner), "mentioned"))
	})

	t.Run("keeps to a member's choices: a channel turned off, a pause, quiet hours", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		d.Owner.PatchJSON("/api/v1/notification-preferences", testkit.JSON{"preferences": []testkit.JSON{
			{"trigger": "new_conversation", "channel": "email", "enabled": false},
		}}).AssertOK()
		d.visitor().start("First")
		d.TC.Drain()
		assert.Empty(t, d.alerts(d.TC.Owner.Email))
		assert.Len(t, filter(bell(t, d.Owner), "new_conversation"), 1)

		d.Owner.Delete("/api/v1/notification-preferences").AssertOK()
		paused := d.Owner.PatchJSON("/api/v1/notification-settings", testkit.JSON{
			"paused_until": d.TC.Clock.Now().Add(time.Hour),
		}).AssertOK().String("data.paused_until")
		until, err := time.Parse(time.RFC3339, paused)
		require.NoError(t, err)
		assert.True(t, until.Equal(d.TC.Clock.Now().Add(time.Hour)))
		d.visitor().start("Second")
		d.TC.Drain()
		assert.Empty(t, d.alerts(d.TC.Owner.Email))
	})

	t.Run("clears the badge on opening the bell and reads everything on request", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		d.visitor().start("One")
		d.visitor().start("Two")
		d.TC.Drain()
		bell(t, d.Owner).AssertJSONPath("meta.unseen_count", float64(2))

		d.Owner.PatchJSON("/api/v1/notifications", testkit.JSON{"seen": true}).AssertOK().
			AssertJSONPath("meta.unseen_count", float64(0)).AssertJSONPath("meta.unread_count", float64(2))
		first := bell(t, d.Owner).String("data.0.id")
		d.Owner.PatchJSON("/api/v1/notifications", testkit.JSON{"ids": []string{first}, "read": true}).AssertOK().
			AssertJSONPath("meta.unread_count", float64(1))
		d.Owner.PatchJSON("/api/v1/notifications", testkit.JSON{"read": true}).AssertOK().
			AssertJSONPath("meta.unread_count", float64(0))
		d.Owner.PatchJSON("/api/v1/notifications", testkit.JSON{"ids": []string{first}, "read": false}).AssertOK().
			AssertJSONPath("meta.unread_count", float64(1))
	})

	t.Run("never shows one member another member's notifications", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		bob, _ := d.Join(t, enums.RoleMember)
		d.visitor().start("Hello")
		d.TC.Drain()
		owner := bell(t, d.Owner).String("data.0.id")
		bob.PatchJSON("/api/v1/notifications", testkit.JSON{"ids": []string{owner}, "read": true}).AssertOK()
		bell(t, d.Owner).AssertJSONPath("meta.unread_count", float64(1))
	})
}

func filter(res *testkit.Response, trigger string) []string {
	var body struct {
		Data []struct {
			ID      string `json:"id"`
			Trigger string `json:"trigger"`
		} `json:"data"`
	}
	res.Decode(&body)
	var out []string
	for _, n := range body.Data {
		if n.Trigger == trigger {
			out = append(out, n.ID)
		}
	}
	return out
}

func itoa(i int) string {
	return strconv.Itoa(i)
}
