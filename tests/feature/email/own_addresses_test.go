package email_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMailThroughOwnAddresses(t *testing.T) {
	t.Parallel()

	t.Run("takes the customer a group in front of the inbox names and keeps the mail open", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)

		deliver(tc, email{
			from: "'Jane Roe' via Support <" + address.Address + ">", to: address.ForwardingAddress,
			subject: "Order 77 never came", id: "group-1@groups.example.com",
			headers: []string{
				"X-Original-Sender: jane@customer.test", "X-Google-Group-Id: 12345",
				"List-Id: <support.acme.test>", "Precedence: list",
			},
			body: "My order never came.",
		}.raw(), "")

		convs := conversations(tc)
		require.Len(t, convs, 1)
		assert.Equal(t, "open", convs[0].Status)
		assert.Equal(t, "normal", emailMetadata(t, messages(tc, convs[0].ID)[0])["classification"])
		tc.Table("contacts").AssertHas(map[string]any{"email": "jane@customer.test", "name": "Jane Roe"})
		tc.Table("contacts").AssertMissing(map[string]any{"email": address.Address})
	})

	t.Run("files the workspace's own mail under the person it went to as a note", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)

		deliver(tc, email{
			from: "Support <" + address.Address + ">", to: "Bob Stone <bob@customer.test>",
			cc: address.ForwardingAddress, subject: "Your refund", id: "own-1@acme.test",
			body: "We sent your refund today.",
		}.raw(), "")

		convs := conversations(tc)
		require.Len(t, convs, 1)
		msgs := messages(tc, convs[0].ID)
		require.Len(t, msgs, 1)
		assert.Equal(t, "note", msgs[0].Type)
		assert.Equal(t, "system", msgs[0].AuthorType)
		assert.Contains(t, *msgs[0].Body, "We sent your refund today.")
		assert.Contains(t, *msgs[0].Body, "Sent from "+address.Address)
		tc.Table("contacts").AssertHas(map[string]any{"email": "bob@customer.test"})
		tc.Table("contacts").AssertMissing(map[string]any{"email": address.Address})
	})

	t.Run("ignores the workspace's own mail that went to nobody outside it", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)

		deliver(tc, email{
			from: address.ForwardingAddress, to: address.ForwardingAddress, subject: "Loop", id: "self-1@acme.test",
			body: "Hello me.",
		}.raw(), "")

		assert.Empty(t, conversations(tc))
		assert.Equal(t, "own_address", *lastInbound(tc, address).IgnoreReason)
	})

	t.Run("keeps a second sender's mail that reuses another mail's message id", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)

		for name, from := range map[string]string{"ann": "Ann <ann@customer.test>", "bob": "Bob <bob@other.test>"} {
			deliver(tc, email{
				from: from, to: address.ForwardingAddress, subject: "Script alert", id: "fixed@script.test",
				body: "Report from " + name,
			}.raw(), to(address.ForwardingAddress, "id", name))
		}

		assert.Len(t, conversations(tc), 2)
	})
}
