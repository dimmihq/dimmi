package email_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
loneMessage delivers a plain text mail with the body and returns the
one message it became.
*/
func loneMessage(t *testing.T, tc *tests.TestCase, body string) models.Message {
	t.Helper()

	address := defaultAddress(tc)
	deliver(tc, email{
		from: "Bob Stone <bob@example.com>", to: address.ForwardingAddress, subject: "Help",
		id: unique("id"), body: strings.ReplaceAll(body, "\n", "\r\n"),
	}.raw(), "")

	convs := conversations(tc)
	require.Len(t, convs, 1)

	msgs := messages(tc, convs[0].ID)
	require.Len(t, msgs, 1)

	return msgs[0]
}

func TestCustomerWordsSurviveIngest(t *testing.T) {
	t.Parallel()

	for name, body := range map[string]string{
		"a sentence that says a manager wrote": "Hi team,\n\nOn Monday my manager wrote:\n" +
			"The invoice must be paid.\nCan you confirm?",
		"a sentence with no colon": "Hi,\n\nOn Monday I wrote\nand nobody answered",
		"a rule of underscores":    "Order details\n______________________\nOrder 123 is missing",
		"a form with from and date": "New contact form\nFrom: Jane\nDate: 2024-01-05\n" +
			"Message: my order never arrived",
		"pasted headers above a question": "Our server shows:\nFrom: billing@example.com\nDate: Jan 3\n" +
			"To: me\nWhy did it fail?",
		"dashes between steps":                    "Steps:\n--\nrun the installer\nreboot\nit crashes",
		"commands in quote marks":                 "Run this:\n> rm -rf build\n> make install",
		"markup a developer writes":               "The <div> tag breaks and <NullPointerException> appears",
		"a list that starts at three":             "3. third step\n4. fourth step",
		"stars and underscores that mean nothing": "2*3*4 equals 24 and _foo_ and [1]: http://example.com",
	} {
		t.Run("keeps "+name+" word for word", func(t *testing.T) {
			t.Parallel()
			tc, _ := emailCase(t)

			m := loneMessage(t, tc, body)

			assert.Equal(t, body, *m.Body)
			assert.Equal(t, "plain", m.BodyFormat)
			assert.Equal(t, body, *m.BodyFull)
		})
	}

	t.Run("keeps what it cuts beside the visible text", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		whole := "Thanks!\n\nOn Tue, Nov 3, 2026 at 10:00 AM Sam <sam@example.com> wrote:\n" +
			"> The refund is on its way.\n\n-- \nBob"

		m := loneMessage(t, tc, whole)

		assert.Equal(t, "Thanks!", *m.Body)
		assert.Equal(t, whole, *m.BodyFull)
	})

	t.Run("shows an excerpt of plain mail without taking words or adding marks", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)

		loneMessage(t, tc, "The <div> tag *breaks*\non 2*3*4")

		tc.ActingAsOwner().GetJSON("/api/v1/conversations?filter[view]=all").AssertOK().
			AssertJSONPath("data.0.last_message.excerpt", "The <div> tag *breaks* on 2*3*4")
	})

	t.Run("shows an excerpt of a teammate's markdown with its words", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		m := loneMessage(t, tc, "Hello")
		path := "/api/v1/conversations/" + models.PublicID(models.PrefixConversation, m.ConversationID)
		owner := tc.ActingAsOwner()

		owner.PostJSON(path+"/replies", testkit.JSON{"body": "Use the <div> tag, **really**"}).AssertCreated().
			AssertJSONPath("data.body_format", "markdown").
			AssertJSONPath("data.has_original", false)
		owner.GetJSON("/api/v1/conversations?filter[view]=all").AssertOK().
			AssertJSONPath("data.0.last_message.excerpt", "Use the <div> tag, really")
	})

	t.Run("says how the timeline should read plain mail", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		m := loneMessage(t, tc, "Hello")
		path := "/api/v1/conversations/" + models.PublicID(models.PrefixConversation, m.ConversationID)

		tc.ActingAsOwner().GetJSON(path+"/messages").AssertOK().
			AssertJSONPath("data.0.body_format", "plain").
			AssertJSONPath("data.0.has_original", true).
			AssertJSONPath("data.0.truncated", false)
	})

	t.Run("quotes the plain words of the customer's message in a reply", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t, mailpitEnv(t))
		address := defaultAddress(tc)
		customer := unique("bob")

		deliver(tc, []byte("From: Bob <"+customer+">\r\nTo: "+address.ForwardingAddress+
			"\r\nSubject: Refund for order 1042\r\n"+
			"Message-ID: <"+unique("id")+">\r\nMIME-Version: 1.0\r\nContent-Type: text/html\r\n\r\n"+
			`<p>The &lt;div&gt; tag is <b>broken</b></p>`), "")

		conversation := conversations(tc)[0]
		_, err := send(tc, memberReply(tc, conversation, customer))
		require.NoError(t, err)

		mails := received(t, customer)
		require.Len(t, mails, 1)
		assert.Contains(t, parsed(t, mails[0]).Text, "> The <div> tag is broken")
	})
}
