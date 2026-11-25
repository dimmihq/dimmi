package email_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
openWeek is open around the clock.
*/
func openWeek() map[string]any {
	day := [][]string{{"00:00", "24:00"}}

	return map[string]any{"mon": day, "tue": day, "wed": day, "thu": day, "fri": day, "sat": day, "sun": day}
}

/*
closedWeek is closed every day.
*/
func closedWeek() map[string]any {
	return map[string]any{}
}

/*
setHours gives the workspace's default office hours the week, in UTC.
*/
func setHours(tc *tests.TestCase, weekly map[string]any) {
	tc.T.Helper()

	owner := tc.ActingAsOwner()
	id := owner.GetJSON("/api/v1/schedules").AssertOK().String("data.0.id")
	owner.PatchJSON("/api/v1/schedules/"+id, map[string]any{"timezone": "UTC", "weekly": weekly}).AssertOK()
}

/*
turnOnAwayReply turns the address's away reply on with the text.
*/
func turnOnAwayReply(tc *tests.TestCase, address models.EmailAddress, text string) {
	tc.T.Helper()

	settings := map[string]any{"away_reply": true}
	if text != "" {
		settings["away_reply_text"] = text
	}

	tc.ActingAsOwner().PatchJSON("/api/v1/email-addresses/"+publicID(address), map[string]any{
		"settings": settings,
	}).AssertOK().AssertJSONPath("data.settings.away_reply", true)
}

/*
newMail delivers a new customer email to the address.
*/
func newMail(t *testing.T, tc *tests.TestCase, address models.EmailAddress) models.Conversation {
	t.Helper()

	deliver(tc, fixture(t, "gmail_new.eml", map[string]string{
		"FORWARDING": address.ForwardingAddress, "TO": address.ForwardingAddress,
	}), to(address.ForwardingAddress)).AssertStatus(202)

	convs := conversations(tc)
	require.Len(tc.T, convs, 1)

	return convs[0]
}

/*
awayReplies are the away replies the conversation got.
*/
func awayReplies(tc *tests.TestCase, conv models.Conversation) []models.Message {
	tc.T.Helper()

	var out []models.Message
	for _, msg := range messages(tc, conv.ID) {
		if strings.Contains(string(msg.Metadata), `"email_away_reply"`) {
			out = append(out, msg)
		}
	}

	return out
}

func TestEmailAwayReply(t *testing.T) {
	t.Parallel()

	t.Run("is off until the address turns it on, with a default text", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)

		tc.ActingAsOwner().GetJSON("/api/v1/email-addresses/"+publicID(address)).AssertOK().
			AssertJSONPath("data.settings.away_reply", false).
			AssertJSONPath("data.settings.away_reply_text", models.DefaultAwayReplyText)
		tc.ActingAsOwner().PatchJSON("/api/v1/email-addresses/"+publicID(address), map[string]any{
			"settings": map[string]any{"away_reply_text": strings.Repeat("a", 1001)},
		}).AssertValidationError("settings.away_reply_text")

		setHours(tc, closedWeek())
		assert.Empty(t, awayReplies(tc, newMail(t, tc, address)))
	})

	t.Run("answers a new email outside office hours once, as the product, saying when the team is back",
		func(t *testing.T) {
			t.Parallel()
			tc, _ := emailCase(t)
			address := defaultAddress(tc)
			setHours(tc, closedWeek())
			turnOnAwayReply(tc, address, "Away for now. Back {{reply_time}}.")

			conv := newMail(t, tc, address)
			replies := awayReplies(tc, conv)
			require.Len(t, replies, 1)
			assert.Equal(t, "Away for now. Back as soon as we can.", *replies[0].Body)
			assert.Equal(t, "system", replies[0].AuthorType)
			assert.False(t, replies[0].Substantive, "an away reply is not an answer")

			after := conversations(tc)[0]
			assert.Nil(t, after.FirstReplyAt, "the conversation still waits for the team")
		})

	t.Run("stays quiet while the team is in office hours", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		setHours(tc, openWeek())
		turnOnAwayReply(tc, address, "")

		assert.Empty(t, awayReplies(tc, newMail(t, tc, address)))
	})

	t.Run("marks a reply the product wrote as automatic, so mail systems do not answer it", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t, mailpitEnv(t))
		address := defaultAddress(tc)
		customer := unique("away")
		r := memberReply(tc, startConversation(tc, address, customer), customer)
		r.outbound.Author = contracts.OutboundAuthor{Type: enums.AuthorTypeSystem}

		_, err := send(tc, r)
		require.NoError(t, err)
		assert.Equal(t, "auto-replied", parsed(t, received(t, customer)[0]).Header.Get("Auto-Submitted"))
	})
}
