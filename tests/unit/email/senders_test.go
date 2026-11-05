package email_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/mailbox"
	"github.com/dimmihq/dimmi/framework/mailbox/authcheck"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/integrations/email"
	"github.com/dimmihq/dimmi/app/models"
)

func TestForwards(t *testing.T) {
	t.Parallel()

	t.Run("reads forwarded blocks of the common clients", func(t *testing.T) {
		t.Parallel()

		for _, text := range []string{
			"FYI\n\n---------- Forwarded message ---------\nFrom: Gina Park <gina@bigco.test>\nSubject: Export\n\nBody",
			"FYI\n\nBegin forwarded message:\n\nFrom: \"Gina Park\" <gina@bigco.test>\nSubject: Export\n\nBody",
			"FYI\n\n________________________________\n" +
				"**Von:** Gina Park <[gina@bigco.test](mailto:gina@bigco.test)>\nBetreff: Export\n\nBody",
		} {
			got, ok := email.FindForward(text)
			require.True(t, ok, text)
			assert.Equal(t, "Gina Park", got.Name)
			assert.Equal(t, "gina@bigco.test", got.Email)
			assert.Equal(t, "Export", got.Subject)
			assert.Equal(t, "FYI", got.Comment)
			assert.Equal(t, "Body", got.Body)
		}
	})

	t.Run("keeps a forward whose sender lost the address", func(t *testing.T) {
		t.Parallel()

		got, ok := email.FindForward("-------- Forwarded Message --------\nFrom: Bob\nDate: today\nBody")
		require.True(t, ok)
		assert.Equal(t, "Bob", got.Name)
		assert.Empty(t, got.Email)

		_, ok = email.FindForward("---------- Forwarded message ---------\nno header here")
		assert.False(t, ok)
	})
}

func TestFormRelays(t *testing.T) {
	t.Parallel()

	service, ok := email.FormRelay("no-reply@webflow.com", nil)
	assert.True(t, ok)
	assert.Equal(t, "webflow.com", service)

	_, ok = email.FormRelay("forms-receipts-noreply@google.com", nil)
	assert.True(t, ok)

	service, ok = email.FormRelay("noreply@forms.example.test", []string{"jules@visitor.test"})
	assert.True(t, ok)
	assert.Equal(t, "forms.example.test", service)

	for _, c := range [][]string{
		{"noreply@a.test", "x@a.test"}, {"bob@a.test", "x@b.test"}, {"noreply@a.test", "nodomain"},
	} {
		_, ok = email.FormRelay(c[0], c[1:])
		assert.False(t, ok, c)
	}

	assert.Equal(t, "jules@visitor.test", email.BodyEmail("Name: Jules\n**Email:** Jules@Visitor.test\n"))
	assert.Empty(t, email.BodyEmail("no address"))
}

func TestConfirmations(t *testing.T) {
	t.Parallel()

	got, ok := email.FindConfirmation("Forwarding-Noreply@google.com", "(#123) Gmail Forwarding Confirmation",
		"click https://mail.google.com/mail/vf-abc to confirm")
	require.True(t, ok)
	assert.Equal(t, email.Confirmation{
		Provider: email.ProviderGmail, Code: "123", ConfirmURL: "https://mail.google.com/mail/vf-abc",
		From: "forwarding-noreply@google.com",
	}, got)

	got, ok = email.FindConfirmation("noreply@zohoaccounts.com", "Confirm forwarding", "Code 4815162342 at "+
		"https://accounts.zoho.eu/confirm?x=1")
	require.True(t, ok)
	assert.Equal(t, "4815162342", got.Code)
	assert.Equal(t, "https://accounts.zoho.eu/confirm?x=1", got.ConfirmURL)

	got, ok = email.FindConfirmation("noreply@zohoaccounts.com", "Forward request", "no code")
	require.True(t, ok)
	assert.Empty(t, got.Code)

	_, ok = email.FindConfirmation("forwarding-noreply@google.com", "Hello", "")
	assert.False(t, ok)
}

func TestAuthentication(t *testing.T) {
	t.Parallel()

	pass := []authcheck.DKIMResult{{Domain: "a.test", Result: authcheck.Pass}}
	cases := []struct {
		name     string
		results  authcheck.Results
		verdicts mailbox.Verdicts
		want     string
		dkim     string
	}{
		{
			"aligned",
			authcheck.Results{DKIM: pass, AlignedDomain: "a.test", DMARC: authcheck.Pass},
			mailbox.Verdicts{},
			email.AuthPass, email.AuthPass,
		},
		{
			"provider fail wins",
			authcheck.Results{AlignedDomain: "a.test"},
			mailbox.Verdicts{DMARC: mailbox.VerdictFail},
			email.AuthFail, email.AuthNone,
		},
		{
			"dns trouble",
			authcheck.Results{DKIM: []authcheck.DKIMResult{{Result: authcheck.TempError}}},
			mailbox.Verdicts{},
			email.AuthUnknown, email.AuthUnknown,
		},
		{"enforced policy", authcheck.Results{
			DMARC: authcheck.Fail, DMARCPolicy: "reject",
			DKIM: []authcheck.DKIMResult{{Result: authcheck.Fail}},
		}, mailbox.Verdicts{}, email.AuthFail, email.AuthFail},
		{"no policy", authcheck.Results{DMARC: authcheck.None}, mailbox.Verdicts{}, email.AuthNone, email.AuthNone},
	}

	for _, c := range cases {
		got := email.Evaluate(c.results, c.verdicts, "test")
		assert.Equal(t, c.want, got.Result, c.name)
		assert.Equal(t, c.dkim, got.DKIM, c.name)
		assert.Equal(t, c.want == email.AuthPass, got.Passed(), c.name)
	}

	assert.Equal(t, authcheck.Pass, email.SPFVerdict(mailbox.VerdictPass))
	assert.Equal(t, authcheck.Fail, email.SPFVerdict(mailbox.VerdictFail))
	assert.Equal(t, authcheck.TempError, email.SPFVerdict(mailbox.VerdictUnknown))
	assert.Equal(t, authcheck.None, email.SPFVerdict(mailbox.VerdictNone))
	assert.Equal(t, authcheck.None, email.SPFVerdict(""))
}

func TestStamps(t *testing.T) {
	t.Parallel()

	stamps := email.NewStamps([]byte("key"))
	loop := stamps.Loop("ws", "id@x.test")
	assert.True(t, stamps.ValidLoop(" "+loop+" ", "ws", "id@x.test"))
	assert.False(t, stamps.ValidLoop(loop, "other", "id@x.test"))
	assert.False(t, stamps.ValidLoop(loop, "ws", ""))
	assert.False(t, email.NewStamps([]byte("other")).ValidLoop(loop, "ws", "id@x.test"))
	assert.True(t, stamps.ValidTest(stamps.Test("address"), "address"))
	assert.False(t, stamps.ValidTest(stamps.Test("address"), "another"))
}

func TestThreadingInputs(t *testing.T) {
	t.Parallel()

	withID := email.Identity{MessageID: "a@x.test"}
	assert.Equal(t, email.DedupeKey(withID), email.DedupeKey(email.Identity{MessageID: "a@x.test", Body: "other"}))
	bare := email.Identity{From: "b@x.test", Subject: "Hi", Body: strings.Repeat("x", 5000)}
	assert.NotEqual(t, email.DedupeKey(withID), email.DedupeKey(bare))
	assert.True(t, strings.HasPrefix(email.SyntheticID(bare), "dimmi-synth-"))
	assert.True(t, strings.HasPrefix(email.DedupeKey(bare), "email:"))
	assert.NotEqual(t, email.DedupeKey(email.Identity{MessageID: "same@x.test", From: "ann@a.test"}),
		email.DedupeKey(email.Identity{MessageID: "same@x.test", From: "bob@b.test"}))
	assert.Equal(t, email.DedupeKey(email.Identity{MessageID: "same@x.test", From: "Ann@A.test"}),
		email.DedupeKey(email.Identity{MessageID: "same@x.test", From: "ann@a.test"}))

	assert.Equal(t, []string{"c", "a"}, email.ReferenceLookups([]string{"b"}, []string{"a", "b", "c"}))
	assert.Equal(t, "root", email.ThreadRoot([]string{"root", "x"}, []string{"y"}, "own"))
	assert.Equal(t, "y", email.ThreadRoot(nil, []string{"y"}, "own"))
	assert.Equal(t, "own", email.ThreadRoot(nil, nil, "own"))
}

func TestComposer(t *testing.T) {
	t.Parallel()

	t.Run("writes the threading and automation headers", func(t *testing.T) {
		t.Parallel()

		msg := email.Compose(email.Draft{
			From: mail.Address{Email: "a@inbound.test"}, To: []mail.Address{{Email: "b@x.test"}}, Subject: "Re: Hi",
			Text: "Hello", HTML: "<p>Hello</p>", MessageID: "m@inbound.test", InReplyTo: "p@x.test",
			References: []string{"r@x.test", "p@x.test"}, Automated: true, LoopStamp: "v1.loop", TestStamp: "v1.test",
			Quote: &email.Quote{Header: "On Mon, Bob wrote:", Text: "Question?\nTwo"},
		})

		assert.Equal(t, "<m@inbound.test>", msg.Headers["Message-ID"])
		assert.Equal(t, "<p@x.test>", msg.Headers["In-Reply-To"])
		assert.Equal(t, "<r@x.test> <p@x.test>", msg.Headers["References"])
		assert.Equal(t, "auto-replied", msg.Headers["Auto-Submitted"])
		assert.Equal(t, "v1.loop", msg.Headers[email.LoopHeader])
		assert.Equal(t, "v1.test", msg.Headers[email.TestHeader])
		assert.Contains(t, msg.Text, "> Question?\n> Two")
		assert.Contains(t, msg.HTML, `<blockquote type="cite"`)

		plain := email.Compose(email.Draft{MessageID: "m@x.test"})
		assert.NotContains(t, plain.Headers, "In-Reply-To")
		assert.NotContains(t, plain.Headers, "Auto-Submitted")
	})

	t.Run("names, subjects and trims for replies", func(t *testing.T) {
		t.Parallel()

		assert.True(t, strings.HasPrefix(email.MessageID("msg_x", "acme.test"), "msg_x."))
		assert.True(t, strings.HasSuffix(email.MessageID("msg_x", "acme.test"), "@acme.test"))
		assert.Equal(t, "Re: Hi", email.ReplySubject("Hi"))
		assert.Equal(t, "RE: Hi", email.ReplySubject("RE: Hi"))
		assert.Equal(t, "Re: Your message", email.ReplySubject(" "))
		assert.Equal(t, []string{"1", "4", "5"}, email.TrimReferences([]string{"1", "2", "3", "4", "5"}, 3))
		assert.Equal(t, []string{"1", "2"}, email.TrimReferences([]string{"1", "2"}, 3))
		assert.Equal(t, "Sam from Acme", email.DisplayName("Sam\r\n from \"Acme\"@<>"))
		assert.Len(t, []rune(email.DisplayName(strings.Repeat("a", 100))), 78)

		zone := time.FixedZone("x", 3600)
		at := time.Date(2026, 11, 2, 9, 4, 0, 0, time.UTC)
		assert.Equal(t, "On Mon, 2 Nov 2026 at 10:04, Bob <b@x.test> wrote:",
			email.QuoteHeader(at, "Bob", "b@x.test", zone))
		assert.Equal(t, "On Mon, 2 Nov 2026 at 10:04, b@x.test wrote:", email.QuoteHeader(at, "", "b@x.test", zone))
	})
}

/*
sender records what the channel hands it.
*/
type sender struct{ sent contracts.OutboundMessage }

/*
Send records the message.
*/
func (s *sender) Send(
	_ context.Context, _ models.ChannelConnection, msg contracts.OutboundMessage,
) (contracts.SendResult, error) {
	s.sent = msg

	return contracts.SendResult{ChannelMessageID: "id"}, nil
}

func TestChannelErrors(t *testing.T) {
	t.Parallel()

	limited := &contracts.RateLimitedError{RetryAfter: time.Minute}
	assert.Equal(t, "channel: rate limited", limited.Error())

	err := &contracts.SendError{Code: "smtp_550", Message: "Unknown user", Err: contracts.ErrRecipientUnreachable}
	assert.Equal(t, "smtp_550: Unknown user", err.Error())
	require.ErrorIs(t, err, contracts.ErrRecipientUnreachable)
}

func TestChannel(t *testing.T) {
	t.Parallel()

	s := &sender{}
	channel := &email.Channel{Sender: s, Transport: "ses"}

	assert.Equal(t, string(enums.ChannelTypeEmail), channel.Type())
	caps := channel.Capabilities(models.ChannelConnection{})
	assert.Equal(t, contracts.TransportPush, caps.Transport)
	assert.True(t, caps.DeliveryReceipts)
	assert.Equal(t, 24*time.Hour, caps.RetryWindow)
	assert.False(t, (&email.Channel{Transport: "smtp"}).Capabilities(models.ChannelConnection{}).DeliveryReceipts)

	got, err := channel.Send(t.Context(), models.ChannelConnection{}, contracts.OutboundMessage{Subject: "Hi"})
	require.NoError(t, err)
	assert.Equal(t, "id", got.ChannelMessageID)
	assert.Equal(t, "Hi", s.sent.Subject)
}
