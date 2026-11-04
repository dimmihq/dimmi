package imap_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/httpclient/safedial"
	"github.com/dimmihq/dimmi/framework/mailbox"
	"github.com/dimmihq/dimmi/framework/mailbox/imap"
	"github.com/dimmihq/dimmi/framework/mailbox/imap/imaptest"
)

const (
	user     = "support@acme.test"
	password = "app-password"
	received = "Received"
	sent     = "Sent Items"
)

func epoch() clock.Clock {
	return clock.NewFake(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
}

func raw(id, body string) []byte {
	return []byte("From: Bob <bob@customer.test>\r\nTo: support@acme.test\r\nSubject: Hello\r\n" +
		"Message-ID: <" + id + "@customer.test>\r\n\r\n" + body + "\r\n")
}

/*
setup starts a server holding the account and returns a client allowed to
reach it.
*/
func setup(t *testing.T, mode imaptest.Mode, change func(*imap.Options)) (*imaptest.Server, *imap.Client) {
	t.Helper()

	server := imaptest.New(t, mode)
	server.AddUser(t, user, password, received, sent)

	dialer, err := safedial.New(safedial.Options{AllowHosts: []string{server.Address()}})
	require.NoError(t, err)

	opts := imap.Options{
		Dialer: dialer, TLSConfig: server.TLSConfig(), Clock: epoch(),
		ReceivedFolder: received, Timeout: 10 * time.Second,
	}
	if change != nil {
		change(&opts)
	}

	client, err := imap.New(opts)
	require.NoError(t, err)

	return server, client
}

func credentials(server *imaptest.Server, security imap.Security) imap.Credentials {
	return imap.Credentials{
		Host: server.Host, Port: server.Port, Security: security, Username: user, Password: password,
	}
}

func open(t *testing.T, server *imaptest.Server, client *imap.Client) *imap.Session {
	t.Helper()

	session, err := client.Open(t.Context(), credentials(server, imap.SecurityTLS))
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })

	return session
}

func TestReading(t *testing.T) {
	t.Parallel()

	t.Run("starts at the newest message so no history is imported", func(t *testing.T) {
		t.Parallel()
		server, client := setup(t, imaptest.TLS, nil)
		server.Deliver(t, user, received, raw("old", "Old."))
		session := open(t, server, client)

		at, err := session.Cursor(t.Context())
		require.NoError(t, err)
		server.Deliver(t, user, received, raw("new", "New."))

		batch, err := session.Changes(t.Context(), at)
		require.NoError(t, err)

		require.Len(t, batch.Messages, 1)
		assert.Equal(t, "new@customer.test", batch.Messages[0].RFCMessageID)
		assert.Equal(t, mailbox.FolderReceived, batch.Messages[0].Folder)
		assert.Equal(t, raw("new", "New."), batch.Messages[0].Raw)
		assert.False(t, batch.More)
	})

	t.Run("reads sent mail before received mail", func(t *testing.T) {
		t.Parallel()
		server, client := setup(t, imaptest.TLS, nil)
		session := open(t, server, client)
		at, err := session.Cursor(t.Context())
		require.NoError(t, err)

		server.Deliver(t, user, received, raw("in", "In."))
		server.Deliver(t, user, sent, raw("out", "Out."))

		batch, err := session.Changes(t.Context(), at)
		require.NoError(t, err)

		require.Len(t, batch.Messages, 2)
		assert.Equal(t, mailbox.FolderSent, batch.Messages[0].Folder)
		assert.Equal(t, mailbox.FolderReceived, batch.Messages[1].Folder)
		assert.Equal(t, sent, session.SentFolder())
	})

	t.Run("leaves every message unread and in place", func(t *testing.T) {
		t.Parallel()
		server, client := setup(t, imaptest.TLS, nil)
		session := open(t, server, client)
		at, err := session.Cursor(t.Context())
		require.NoError(t, err)
		server.Deliver(t, user, received, raw("a", "A."))

		_, err = session.Changes(t.Context(), at)
		require.NoError(t, err)

		held := server.Messages(t, user, password, received)
		require.Len(t, held, 1)
		assert.False(t, held[0].Seen())
	})

	t.Run("reads a batch at a time and moves the cursor past it", func(t *testing.T) {
		t.Parallel()
		server, client := setup(t, imaptest.TLS, func(o *imap.Options) { o.BatchSize = 2 })
		session := open(t, server, client)
		at, err := session.Cursor(t.Context())
		require.NoError(t, err)
		for _, id := range []string{"a", "b", "c"} {
			server.Deliver(t, user, received, raw(id, id))
		}

		first, err := session.Changes(t.Context(), at)
		require.NoError(t, err)
		second, err := session.Changes(t.Context(), first.Cursor)
		require.NoError(t, err)
		third, err := session.Changes(t.Context(), second.Cursor)
		require.NoError(t, err)

		assert.Len(t, first.Messages, 2)
		assert.True(t, first.More)
		require.Len(t, second.Messages, 1)
		assert.Equal(t, "c@customer.test", second.Messages[0].RFCMessageID)
		assert.False(t, second.More)
		assert.Empty(t, third.Messages)
	})

	t.Run("reads a message too large as its header only", func(t *testing.T) {
		t.Parallel()
		server, client := setup(t, imaptest.TLS, func(o *imap.Options) { o.MaxMessageBytes = 200 })
		session := open(t, server, client)
		at, err := session.Cursor(t.Context())
		require.NoError(t, err)
		big := raw("big", strings.Repeat("x", 500))
		server.Deliver(t, user, received, big)

		batch, err := session.Changes(t.Context(), at)
		require.NoError(t, err)

		require.Len(t, batch.Messages, 1)
		message := batch.Messages[0]
		assert.True(t, message.Truncated)
		assert.Equal(t, int64(len(big)), message.Size)
		assert.True(t, bytes.HasPrefix(big, message.Raw))
		assert.NotContains(t, string(message.Raw), "xxx")
	})

	t.Run("reads a renumbered folder again from before the last sync", func(t *testing.T) {
		t.Parallel()
		server, client := setup(t, imaptest.TLS, nil)
		server.Deliver(t, user, received, raw("kept", "Kept."))
		session := open(t, server, client)
		at, err := session.Cursor(t.Context())
		require.NoError(t, err)
		require.NoError(t, session.Close())

		server.Renumber(t, user, password, received)
		server.Deliver(t, user, received, raw("after", "After."))

		batch, err := open(t, server, client).Changes(t.Context(), at)
		require.NoError(t, err)

		ids := []string{}
		for _, message := range batch.Messages {
			ids = append(ids, message.RFCMessageID)
		}
		assert.Equal(t, []string{"kept@customer.test", "after@customer.test"}, ids)
	})

	t.Run("starts a folder it has not seen at its newest message", func(t *testing.T) {
		t.Parallel()
		server, client := setup(t, imaptest.TLS, nil)
		server.Deliver(t, user, sent, raw("old-sent", "Old."))
		session := open(t, server, client)

		batch, err := session.Changes(t.Context(), []byte(`{}`))
		require.NoError(t, err)

		assert.Empty(t, batch.Messages)
		assert.Contains(t, string(batch.Cursor), `"uidvalidity"`)
	})

	t.Run("tells a cursor it cannot read as stale", func(t *testing.T) {
		t.Parallel()
		server, client := setup(t, imaptest.TLS, nil)

		_, err := open(t, server, client).Changes(t.Context(), []byte("nonsense"))

		require.ErrorIs(t, err, mailbox.ErrCursorExpired)
	})
}

func TestAppend(t *testing.T) {
	t.Parallel()

	t.Run("files a sent message into sent, read", func(t *testing.T) {
		t.Parallel()
		server, client := setup(t, imaptest.TLS, nil)

		require.NoError(t, open(t, server, client).Append(t.Context(), raw("reply", "Thanks.")))

		held := server.Messages(t, user, password, sent)
		require.Len(t, held, 1)
		assert.True(t, held[0].Seen())
		assert.Equal(t, raw("reply", "Thanks."), held[0].Raw)
	})

	t.Run("reports a mailbox without a sent folder", func(t *testing.T) {
		t.Parallel()
		server := imaptest.New(t, imaptest.TLS)
		server.AddUser(t, user, password, received)
		dialer, err := safedial.New(safedial.Options{AllowHosts: []string{server.Address()}})
		require.NoError(t, err)
		client, err := imap.New(imap.Options{
			Dialer: dialer, TLSConfig: server.TLSConfig(), Clock: epoch(), ReceivedFolder: received,
		})
		require.NoError(t, err)

		session := open(t, server, client)
		_, err = session.Cursor(t.Context())
		require.NoError(t, err)

		assert.Empty(t, session.SentFolder())
		require.ErrorIs(t, session.Append(t.Context(), raw("r", "r")), imap.ErrNoSentFolder)
	})
}

func TestBrokenConnection(t *testing.T) {
	t.Parallel()

	t.Run("reports every call after the connection broke", func(t *testing.T) {
		t.Parallel()
		server, client := setup(t, imaptest.TLS, nil)
		session := open(t, server, client)
		at, err := session.Cursor(t.Context())
		require.NoError(t, err)
		require.NoError(t, session.Close())

		_, err = session.Cursor(t.Context())
		require.ErrorIs(t, err, imap.ErrUnreachable)
		_, err = session.Changes(t.Context(), at)
		require.ErrorIs(t, err, imap.ErrUnreachable)
		require.ErrorIs(t, session.Append(t.Context(), raw("r", "r")), imap.ErrUnreachable)
	})
}

func TestOpening(t *testing.T) {
	t.Parallel()

	t.Run("refuses a wrong password", func(t *testing.T) {
		t.Parallel()
		server, client := setup(t, imaptest.TLS, nil)
		creds := credentials(server, imap.SecurityTLS)
		creds.Password = "wrong"

		_, err := client.Open(t.Context(), creds)

		require.ErrorIs(t, err, imap.ErrAuthFailed)
	})

	t.Run("signs in over starttls", func(t *testing.T) {
		t.Parallel()
		server, client := setup(t, imaptest.StartTLS, nil)

		session, err := client.Open(t.Context(), credentials(server, imap.SecurityStartTLS))
		require.NoError(t, err)

		require.NoError(t, session.Close())
	})

	t.Run("refuses a server that offers no encryption", func(t *testing.T) {
		t.Parallel()
		server, client := setup(t, imaptest.Plain, nil)

		_, err := client.Open(t.Context(), credentials(server, imap.SecurityTLS))
		require.ErrorIs(t, err, imap.ErrTLSRequired)

		_, err = client.Open(t.Context(), credentials(server, imap.SecurityStartTLS))
		require.ErrorIs(t, err, imap.ErrTLSRequired)
	})

	t.Run("refuses a host on the private network", func(t *testing.T) {
		t.Parallel()
		server, _ := setup(t, imaptest.TLS, nil)
		dialer, err := safedial.New(safedial.Options{})
		require.NoError(t, err)
		client, err := imap.New(imap.Options{Dialer: dialer, Clock: epoch(), ReceivedFolder: received})
		require.NoError(t, err)

		_, err = client.Open(t.Context(), credentials(server, imap.SecurityTLS))

		require.ErrorIs(t, err, imap.ErrHostBlocked)
	})

	t.Run("reports a server that cannot be reached", func(t *testing.T) {
		t.Parallel()
		dialer, err := safedial.New(safedial.Options{AllowHosts: []string{"127.0.0.1:1"}})
		require.NoError(t, err)
		client, err := imap.New(imap.Options{Dialer: dialer, Clock: epoch(), ReceivedFolder: received})
		require.NoError(t, err)

		_, err = client.Open(t.Context(), imap.Credentials{Host: "127.0.0.1", Port: 1, Username: user})

		require.ErrorIs(t, err, imap.ErrUnreachable)
	})

	t.Run("needs a dialer, a clock and a received folder", func(t *testing.T) {
		t.Parallel()

		_, err := imap.New(imap.Options{})

		require.ErrorIs(t, err, imap.ErrInvalidOptions)
	})
}
