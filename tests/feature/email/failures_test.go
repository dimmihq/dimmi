package email_test

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/mailbox"
	"github.com/dimmihq/dimmi/framework/testkit"

	emailactions "github.com/dimmihq/dimmi/app/actions/email"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
breakage breaks one piece of storage: the writes of the kinds to a
table, or the whole table when no kind is named.
*/
type breakage struct {
	table string
	kinds []string
}

/*
apply breaks the storage for the rest of the test.
*/
func (b breakage) apply(tc *tests.TestCase) {
	if len(b.kinds) == 0 {
		tc.BreakTable(b.table)

		return
	}

	tc.FailWrites(b.table, b.kinds...)
}

/*
name names the breakage for its subtest.
*/
func (b breakage) name() string {
	if len(b.kinds) == 0 {
		return b.table + " gone"
	}

	return b.table + " " + b.kinds[0]
}

func TestInboundStorageFailures(t *testing.T) {
	t.Parallel()

	t.Run("fails the email and starts no conversation when a write of the first message fails", func(t *testing.T) {
		t.Parallel()

		for _, b := range []breakage{
			{"email_message_refs", []string{"INSERT"}},
			{"email_threads", []string{"INSERT"}},
			{"conversations", []string{"INSERT"}},
			{"conversations", []string{"UPDATE"}},
			{"contacts", []string{"INSERT"}},
			{"contact_identities", []string{"INSERT"}},
			{"messages", []string{"INSERT"}},
			{"conversation_participants", []string{"INSERT"}},
			{"attachments", []string{"INSERT"}},
			{"email_addresses", []string{"UPDATE"}},
			{"workspaces", []string{"UPDATE"}},
			{"email_message_refs", nil},
			{"workspace_members", nil},
			{"contact_identities", nil},
		} {
			t.Run(b.name(), func(t *testing.T) {
				tc, _ := emailCase(t)
				address := defaultAddress(tc)
				b.apply(tc)

				deliver(tc, fixture(t, "attachments.eml", map[string]string{"TO": address.ForwardingAddress}), "")

				row := lastInbound(tc, address)
				assert.Equal(t, "failed", row.Status)
				assert.NotEmpty(t, *row.Error)
				tc.Table("messages").AssertCount(0)
			})
		}
	})

	t.Run("fails a reply when the thread it joins cannot be written", func(t *testing.T) {
		t.Parallel()

		for _, b := range []breakage{
			{"messages", []string{"UPDATE"}},
			{"conversation_participants", nil},
			{"email_threads", nil},
		} {
			t.Run(b.name(), func(t *testing.T) {
				tc, _ := emailCase(t)
				address := defaultAddress(tc)
				conversation := startConversation(tc, address, "first@customer.test")
				token := replyToken(tc, address, conversation.ID)
				local := address.ForwardingAddress[:len(address.ForwardingAddress)-len("@"+inboundDomain)]
				b.apply(tc)

				tokenAddress := local + "+" + token + "@" + inboundDomain
				deliver(tc, email{
					from: "Other <other@customer.test>", to: tokenAddress, subject: "Re: Refund for order 1042",
					id: "stranger@customer.test", body: "Me too.",
				}.raw(), to(tokenAddress))

				assert.Equal(t, "failed", lastInbound(tc, address).Status)
			})
		}
	})

	t.Run("keeps processing when the sender counters are away", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		tc.BreakValkey()

		deliver(tc, fixture(t, "gmail_new.eml", map[string]string{"TO": address.ForwardingAddress}), "")

		assert.Equal(t, "processed", lastInbound(tc, address).Status)
	})

	t.Run("cannot even start when its own row cannot be claimed", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		tc.FailWrites("inbound_emails", "UPDATE")

		deliver(tc, fixture(t, "gmail_new.eml", map[string]string{"TO": address.ForwardingAddress}), "")

		assert.Equal(t, "received", lastInbound(tc, address).Status)
		assert.Empty(t, conversations(tc))
	})

	t.Run("answers the provider with an error when the email cannot be recorded", func(t *testing.T) {
		t.Parallel()

		for _, b := range []breakage{{"inbound_emails", []string{"INSERT"}}, {"river_job", []string{"INSERT"}}} {
			t.Run(b.name(), func(t *testing.T) {
				tc, _ := emailCase(t)
				address := defaultAddress(tc)
				b.apply(tc)

				deliver(tc, fixture(t, "gmail_new.eml", map[string]string{"TO": address.ForwardingAddress}),
					to(address.ForwardingAddress)).AssertStatus(500)
			})
		}
	})

	t.Run("fails a delivery that cannot open its raw message", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)

		_, err := tc.App.Actions.ReceiveInboundEmail.Handle(t.Context(), mailbox.Envelope{
			Source: "test", Recipients: []string{address.ForwardingAddress},
			OpenRaw: func(context.Context) (io.ReadCloser, error) { return nil, assert.AnError },
		})
		require.ErrorIs(t, err, assert.AnError)

		raw := &mailbox.RawRef{Disk: "local", Path: "elsewhere.eml"}
		received, err := tc.App.Actions.ReceiveInboundEmail.Handle(t.Context(), mailbox.Envelope{
			Source: "test", ProviderMessageID: "ref-1", Recipients: []string{address.ForwardingAddress}, RawRef: raw,
		})
		require.NoError(t, err)
		require.Len(t, received, 1)

		tc.Drain()
		assert.Equal(t, "failed", lastInbound(tc, address).Status)
	})
}

/*
reports returns how many of the bounce and the complaint the breakage
fails: a complaint never touches the reply's message.
*/
func reports(b breakage) int {
	if b.table == "messages" {
		return 1
	}

	return 2
}

func TestReportStorageFailures(t *testing.T) {
	t.Parallel()

	for _, b := range []breakage{
		{"email_suppressions", []string{"INSERT"}},
		{"outbound_emails", []string{"UPDATE"}},
		{"messages", []string{"UPDATE"}},
		{"outbound_emails", nil},
	} {
		t.Run("fails a bounce it cannot record: "+b.name(), func(t *testing.T) {
			t.Parallel()
			tc, _ := emailCase(t)
			address := defaultAddress(tc)
			customer := unique("bounce")
			result, err := send(tc, memberReply(tc, startConversation(tc, address, customer), customer))
			require.NoError(t, err)
			b.apply(tc)

			for _, name := range []string{"bounce_dsn.eml", "complaint_arf.eml"}[:reports(b)] {
				deliver(tc, fixture(t, name, map[string]string{
					"TO": address.ForwardingAddress, "FORWARDING": address.ForwardingAddress, "RECIPIENT": customer,
					"ORIGINAL": result.ChannelMessageID,
				}), to(address.ForwardingAddress, "id", name))

				assert.Equal(t, "failed", lastInbound(tc, address).Status, name)
			}
		})
	}

	t.Run("fails a notice it cannot keep", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		tc.FailWrites("email_addresses", "UPDATE")

		deliver(tc, fixture(t, "gmail_confirmation.eml", map[string]string{
			"TO": address.ForwardingAddress, "FORWARDING": address.ForwardingAddress,
		}), "")

		assert.Equal(t, "failed", lastInbound(tc, address).Status)
	})
}

func TestOutboundStorageFailures(t *testing.T) {
	t.Parallel()

	for _, b := range []breakage{
		{"outbound_emails", []string{"INSERT"}},
		{"outbound_emails", []string{"UPDATE"}},
		{"outbound_emails", nil},
		{"email_threads", []string{"INSERT"}},
		{"email_message_refs", nil},
		{"email_message_refs", []string{"INSERT"}},
		{"email_suppressions", nil},
		{"email_addresses", []string{"UPDATE"}},
		{"email_addresses", nil},
		{"messages", nil},
		{"workspaces", nil},
	} {
		t.Run("fails a reply it cannot record: "+b.name(), func(t *testing.T) {
			t.Parallel()
			tc, _ := emailCase(t)
			address := defaultAddress(tc)
			customer := unique("fail")
			r := memberReply(tc, startConversation(tc, address, customer), customer)
			b.apply(tc)

			_, err := send(tc, r)
			require.Error(t, err)
		})
	}
}

func TestAddressStorageFailures(t *testing.T) {
	t.Parallel()

	t.Run("answers a server error when the addresses cannot be read or written", func(t *testing.T) {
		t.Parallel()

		for _, b := range []breakage{
			{"email_addresses", nil},
			{"email_addresses", []string{"INSERT", "UPDATE"}},
			{"channel_connections", []string{"INSERT", "UPDATE"}},
			{"inbound_emails", nil},
			{"email_address_tests", []string{"INSERT"}},
			{"inboxes", nil},
		} {
			t.Run(b.name(), func(t *testing.T) {
				tc, _ := emailCase(t)
				id := publicID(defaultAddress(tc))
				owner := tc.ActingAsOwner()
				owner.GetJSON("/api/v1/workspace").AssertOK()
				b.apply(tc)

				statuses := []int{
					owner.GetJSON("/api/v1/email-addresses").Status,
					owner.GetJSON("/api/v1/email-addresses/" + id).Status,
					owner.PostJSON("/api/v1/email-addresses", testkit.JSON{"address": "new@acme.test"}).Status,
					owner.PatchJSON("/api/v1/email-addresses/"+id, testkit.JSON{"display_name": "X"}).Status,
					owner.PostJSON("/api/v1/email-addresses/"+id+"/tests", nil).Status,
					owner.GetJSON("/api/v1/email-addresses/" + id + "/inbound-emails").Status,
					owner.Delete("/api/v1/email-addresses/" + id).Status,
				}
				assert.Contains(t, statuses, 500)
			})
		}
	})

	t.Run("creates no workspace when its first address cannot be made", func(t *testing.T) {
		t.Parallel()

		for _, b := range []breakage{
			{"email_addresses", nil},
			{"email_addresses", []string{"INSERT"}},
		} {
			t.Run(b.name(), func(t *testing.T) {
				tc, _ := emailCase(t)
				user := tc.CreateUser()
				b.apply(tc)

				_, err := factories.Workspace(t.Context(), tc.App, user)
				require.ErrorContains(t, err, "default_email_address")
			})
		}
	})

	t.Run("refuses every action to a member without email permissions", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		m := authz.NewMembership(tc.Workspace.Workspace, tc.Workspace.Owner, nil)
		acts := tc.App.Actions
		ctx := t.Context()

		_, err := acts.ListEmailAddresses.Handle(ctx, m)
		require.ErrorIs(t, err, exceptions.ErrForbidden)
		_, err = acts.ShowEmailAddress.Handle(ctx, m, address.ID)
		require.ErrorIs(t, err, exceptions.ErrForbidden)
		_, err = acts.CreateEmailAddress.Handle(ctx, emailactions.CreateEmailAddressInput{
			Membership: m, Address: "a@b.test",
		})
		require.ErrorIs(t, err, exceptions.ErrForbidden)
		_, err = acts.UpdateEmailAddress.Handle(ctx, emailactions.UpdateEmailAddressInput{
			Membership: m, ID: address.ID,
		})
		require.ErrorIs(t, err, exceptions.ErrForbidden)
		require.ErrorIs(t, acts.DeleteEmailAddress.Handle(ctx, m, address.ID), exceptions.ErrForbidden)
		_, err = acts.SendEmailAddressTest.Handle(ctx, m, address.ID)
		require.ErrorIs(t, err, exceptions.ErrForbidden)
		_, err = acts.ListInboundEmails.Handle(ctx, emailactions.ListInboundEmailsInput{
			Membership: m, AddressID: address.ID,
		})
		require.ErrorIs(t, err, exceptions.ErrForbidden)
	})

	t.Run("refuses a public address with a name and fits long display names", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		m := authz.NewMembership(tc.Workspace.Workspace, tc.Workspace.Owner, []string{"email:manage", "email:read"})
		acts := tc.App.Actions

		_, err := acts.CreateEmailAddress.Handle(t.Context(), emailactions.CreateEmailAddressInput{
			Membership: m, Address: "Support <support@acme.test>",
		})
		require.ErrorIs(t, err, exceptions.ErrInvalidEmailAddress)

		long := strings.Repeat("x", 100)

		created, err := acts.CreateEmailAddress.Handle(t.Context(), emailactions.CreateEmailAddressInput{
			Membership: m, Address: "long@acme.test", DisplayName: long,
		})
		require.NoError(t, err)
		assert.Len(t, created.DisplayName, 78)

		blank := " "
		row, err := acts.UpdateEmailAddress.Handle(t.Context(), emailactions.UpdateEmailAddressInput{
			Membership: m, ID: created.ID, DisplayName: &blank,
		})
		require.NoError(t, err)
		assert.Equal(t, "Support", row.EmailAddress.DisplayName)
		assert.Equal(t, "default_email_address", (&emailactions.EnsureDefaultEmailAddress{}).Name())
	})
}
