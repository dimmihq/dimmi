package broadcasts_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
)

func TestEmailBroadcasts(t *testing.T) {
	t.Parallel()

	t.Run("sends each person their own email from the connected address with a way out", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		ada, bob := unique("ada"), unique("bob")
		d.contact(ada, "Ada Okafor", "pro")
		d.contact(bob, "Bob", "pro")
		d.contact("", "No Email", "pro")
		d.contact(unique("free"), "Free Person", "free")

		id := d.emailBroadcast("November update", nil)
		d.send(id)

		headers, text := mailFor(t, ada)
		assert.Contains(t, headers, "support@acme.test")
		assert.Contains(t, headers, `"List-Unsubscribe-Post":["List-Unsubscribe=One-Click"]`)
		assert.Contains(t, headers, "/b/u/")
		assert.Contains(t, headers, "+")
		assert.Contains(t, headers, "What's new for Ada")
		assert.Contains(t, text, "Hi Ada,")
		assert.Contains(t, text, "Unsubscribe or choose what you get")

		report := d.report(id)
		report.AssertJSONPath("data.broadcast.status", "sent")
		report.AssertJSONPath("data.counts.sent", float64(2))
		report.AssertJSONPath("data.counts.skipped", float64(1))
		report.AssertJSONPath("data.reasons.0.reason", "no_email")
		_, _ = mailFor(t, bob)
	})

	t.Run("opens a conversation that starts with the broadcast when someone replies", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		ada := unique("ada")
		d.contact(ada, "Ada Okafor", "pro")
		id := d.emailBroadcast("Webinar", testkit.JSON{
			"reply_member_id": models.PublicID(models.PrefixMember, d.tc.Workspace.Owner.ID),
		})
		d.send(id)

		r := d.recipients(id)[0]
		require.NotNil(t, r.MessageIDHeader)

		raw := "From: Ada Okafor <" + ada + ">\r\nTo: Acme <" + d.forwarding + ">\r\nSubject: Re: What's new\r\n" +
			"Message-ID: <reply." + ids.New().String() + "@customer.test>\r\n" +
			"In-Reply-To: <" + *r.MessageIDHeader + ">\r\n" +
			"\r\nCount me in for Thursday.\r\n"
		replyTo := strings.Replace(d.forwarding, "@", "+"+r.ReplyToken+"@", 1)
		d.deliver(raw, replyTo)

		r = d.recipients(id)[0]
		require.NotNil(t, r.ConversationID)
		assert.NotNil(t, r.RepliedAt)

		convID := models.PublicID(models.PrefixConversation, *r.ConversationID)
		conv := d.owner.GetJSON("/api/v1/conversations/" + convID + "/messages").AssertOK()
		conv.AssertJSONPath("data.0.body", "Hi Ada,\n\nRead [the changelog](https://acme.test/changelog).")
		assert.Contains(t, string(conv.Body), "Count me in for Thursday.")

		d.report(id).AssertJSONPath("data.counts.replied", float64(1))
	})

	t.Run("refuses to send from an address whose domain is not verified", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		other := d.owner.PostJSON("/api/v1/email-addresses", testkit.JSON{"address": "hello@other.test"}).
			AssertCreated().String("data.id")
		d.contact(unique("ada"), "Ada", "pro")
		id := d.emailBroadcast("Unverified", testkit.JSON{"email_address_id": other})

		d.owner.PostJSON("/api/v1/broadcasts/"+id+"/send", nil).AssertStatus(422).
			AssertJSONPath("code", "broadcast_address_unverified")
	})

	t.Run("honors a one-click unsubscribe and leaves the person out next time", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		ada := unique("ada")
		d.contact(ada, "Ada", "pro")
		id := d.emailBroadcast("First", testkit.JSON{"skip_frequency_cap": true})
		d.send(id)

		token := tokenOf(t, ada)
		d.tc.Client().Send("POST", "/b/u/"+token+"/one-click", []byte("List-Unsubscribe=One-Click")).AssertOK()
		d.drain()
		d.report(id).AssertJSONPath("data.counts.unsubscribed", float64(1))

		next := d.emailBroadcast("Second", testkit.JSON{"skip_frequency_cap": true})
		d.send(next)
		d.report(next).AssertJSONPath("data.counts.sent", float64(0)).
			AssertJSONPath("data.reasons.0.reason", "unsubscribed")
	})

	t.Run("lets the person read and save their choices from the link, leaving them out next time", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		ada := unique("ada")
		d.contact(ada, "Ada", "pro")
		d.send(d.emailBroadcast("First", testkit.JSON{"skip_frequency_cap": true}))
		page := "/api/v1/broadcast-preferences/" + tokenOf(t, ada)

		d.tc.Client().GetJSON(page).AssertOK().AssertJSONPath("data.unsubscribed_all", false)
		d.tc.Client().PatchJSON(page, testkit.JSON{"topics": []string{}, "unsubscribe_all": true}).AssertOK().
			AssertJSONPath("data.unsubscribed_all", true)
		d.tc.Client().GetJSON(page).AssertOK().AssertJSONPath("data.unsubscribed_all", true)
		d.tc.Client().GetJSON("/api/v1/broadcast-preferences/not-a-token").AssertNotFound()

		next := d.emailBroadcast("Second", testkit.JSON{"skip_frequency_cap": true})
		d.send(next)
		d.report(next).AssertJSONPath("data.counts.sent", float64(0)).
			AssertJSONPath("data.reasons.0.reason", "unsubscribed")
	})

	t.Run("leaves out someone who got a broadcast too recently", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		d.contact(unique("ada"), "Ada", "pro")
		d.send(d.emailBroadcast("First", nil))

		next := d.emailBroadcast("Second", nil)
		d.send(next)
		d.report(next).AssertJSONPath("data.reasons.0.reason", "frequency_cap")
	})

	t.Run("pauses itself when too many emails bounce", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t, "DIMMI_BROADCASTS__PAUSE_AFTER=2", "DIMMI_BROADCASTS__EMAIL_PER_MINUTE=2",
			"DIMMI_BROADCASTS__DISPATCH_SECONDS=60")
		for i := range 6 {
			d.contact(unique("gone"+string(rune('a'+i))), "Gone", "pro")
		}

		id := d.emailBroadcast("Bouncy", nil)
		d.owner.PostJSON("/api/v1/broadcasts/"+id+"/send", nil).AssertOK()
		d.tc.Drain()

		for _, r := range d.recipients(id) {
			if r.MessageIDHeader == nil {
				continue
			}

			d.tc.Client().WithHeader("Authorization", "Bearer "+inboundSecret).
				Send("POST", "/webhooks/test/inbound?to="+d.forwarding,
					[]byte(bounce(d.forwarding, r.Address, *r.MessageIDHeader))).AssertStatus(202)
			d.tc.Drain()
		}

		d.drain()

		report := d.report(id)
		report.AssertJSONPath("data.broadcast.status", "paused")
		report.AssertJSONPath("data.broadcast.pause_reason", "bounce_rate")
		report.AssertJSONPath("data.reasons.0.reason", "address_not_found")
		assert.NotEqual(t, "0", report.String("data.counts.bounced"))
	})
}

/*
tokenOf reads the link token in the unsubscribe header the person got.
*/
func tokenOf(t *testing.T, recipient string) string {
	t.Helper()

	headers, _ := mailFor(t, recipient)
	_, rest, ok := strings.Cut(headers, "/b/u/")
	require.True(t, ok)
	token, _, _ := strings.Cut(rest, "/one-click")

	return token
}

/*
bounce is a delivery report saying the address does not exist.
*/
func bounce(forwarding, recipient, original string) string {
	host := "mx" + strings.ToLower(ids.New().String()[:8]) + ".test"

	return "From: Mail Delivery Subsystem <mailer-daemon@" + host + ">\r\nTo: " + forwarding + "\r\n" +
		"Subject: Delivery Status Notification (Failure)\r\nMessage-ID: <bounce." + ids.New().String() +
		"@" + host + ">\r\nMIME-Version: 1.0\r\n" +
		"Content-Type: multipart/report; report-type=delivery-status; boundary=\"b\"\r\n\r\n" +
		"--b\r\nContent-Type: text/plain\r\n\r\nNot delivered.\r\n\r\n" +
		"--b\r\nContent-Type: message/delivery-status\r\n\r\nReporting-MTA: dns; " + host + "\r\n\r\n" +
		"Final-Recipient: rfc822; " + recipient + "\r\nAction: failed\r\nStatus: 5.1.1\r\n" +
		"Diagnostic-Code: smtp; 550 5.1.1 no such user\r\n\r\n" +
		"--b\r\nContent-Type: text/rfc822-headers\r\n\r\nFrom: Acme <support@acme.test>\r\nTo: " + recipient +
		"\r\nSubject: Bouncy\r\nMessage-ID: <" + original + ">\r\n\r\n--b--\r\n"
}
