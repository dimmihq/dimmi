package email_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
)

func TestReportsStopOnlyTheRightMail(t *testing.T) {
	t.Parallel()

	t.Run("ignores a report naming an address the email never went to", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		customer := unique("forger")
		r := memberReply(tc, startConversation(tc, address, customer), customer)
		result, err := send(tc, r)
		require.NoError(t, err)

		deliver(tc, fixture(t, "bounce_dsn.eml", map[string]string{
			"TO": address.ForwardingAddress, "FORWARDING": address.ForwardingAddress, "RECIPIENT": "ceo@victim.test",
			"ORIGINAL": result.ChannelMessageID,
		}), "")

		tc.Table("email_suppressions").AssertCount(0)
		tc.Table("messages").AssertMissing(map[string]any{"id": r.message.ID.String(), "delivery_status": "failed"})
	})

	t.Run("stops mail for a few days only when a mailbox refuses it for another reason", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		customer := unique("full")
		conversation := startConversation(tc, address, customer)
		result, err := send(tc, memberReply(tc, conversation, customer))
		require.NoError(t, err)

		raw := fixture(t, "bounce_dsn.eml", map[string]string{
			"TO": address.ForwardingAddress, "FORWARDING": address.ForwardingAddress, "RECIPIENT": customer,
			"ORIGINAL": result.ChannelMessageID,
		})
		deliver(tc, []byte(strings.ReplaceAll(string(raw), "5.1.1", "5.2.2")), "")

		tc.Table("email_suppressions").AssertHas(map[string]any{"email": customer, "reason": "soft_bounce"})
		_, err = send(tc, memberReply(tc, conversation, customer))
		require.ErrorIs(t, err, contracts.ErrRecipientUnreachable)

		tc.Clock.Travel(73 * time.Hour)
		again, err := send(tc, memberReply(tc, conversation, customer))
		require.NoError(t, err)
		assert.Equal(t, enums.DeliveryStatusSent, again.Status)
	})

	t.Run("mails a customer again once they write in after a bounce", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		customer := unique("back")
		conversation := startConversation(tc, address, customer)
		result, err := send(tc, memberReply(tc, conversation, customer))
		require.NoError(t, err)

		deliver(tc, fixture(t, "bounce_dsn.eml", map[string]string{
			"TO": address.ForwardingAddress, "FORWARDING": address.ForwardingAddress, "RECIPIENT": customer,
			"ORIGINAL": result.ChannelMessageID,
		}), "")
		tc.Table("email_suppressions").AssertHas(map[string]any{"email": customer, "reason": "hard_bounce"})

		deliver(tc, email{
			from: "Bob Stone <" + customer + ">", to: address.ForwardingAddress, subject: "My mailbox works again",
			id: "back." + customer, body: "Sorry, my mailbox was broken.",
		}.raw(), to(address.ForwardingAddress))

		tc.Table("email_suppressions").AssertCount(0)
		_, err = send(tc, memberReply(tc, conversation, customer))
		require.NoError(t, err)
	})

	t.Run("stops mail only to the person who complained", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		customer, boss := unique("calm"), unique("boss")
		r := memberReply(tc, startConversation(tc, address, customer), customer)
		r.outbound.CC = []contracts.Recipient{{IdentityType: enums.IdentityTypeEmail, IdentityValue: boss}}
		result, err := send(tc, r)
		require.NoError(t, err)

		raw := fixture(t, "complaint_arf.eml", map[string]string{
			"TO": address.ForwardingAddress, "FORWARDING": address.ForwardingAddress, "RECIPIENT": customer,
			"ORIGINAL": result.ChannelMessageID,
		})
		named := strings.Replace(string(raw), "Original-Rcpt-To: <"+customer, "Original-Rcpt-To: <"+boss, 1)
		deliver(tc, []byte(named), "")

		tc.Table("email_suppressions").AssertHas(map[string]any{"email": boss, "reason": "complaint"})
		tc.Table("email_suppressions").AssertMissing(map[string]any{"email": customer})
	})
}
