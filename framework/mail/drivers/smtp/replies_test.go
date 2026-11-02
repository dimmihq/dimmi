package smtp_test

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/mail/drivers/smtp"
	"github.com/dimmihq/dimmi/framework/mail/drivertest"
)

/*
script is how a scripted server answers each step of one transaction.
*/
type script struct {
	mail, rcpt, data, final string
}

/*
serve starts a server on a free local port that answers one connection
from the script, standing in for relays whose refusals Mailpit never
sends.
*/
func serve(t *testing.T, s script) smtp.Options {
	t.Helper()

	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		answer(conn, s)
	}()

	host, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	number, err := strconv.Atoi(port)
	require.NoError(t, err)

	return smtp.Options{Host: host, Port: number, AllowInsecure: true}
}

func answer(conn net.Conn, s script) {
	r := bufio.NewReader(conn)
	reply := func(line string) { _, _ = conn.Write([]byte(line + "\r\n")) }
	reply("220 test ready")

	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}

		switch verb := strings.ToUpper(strings.Fields(line + " x")[0]); verb {
		case "EHLO", "HELO", "RSET", "NOOP":
			reply("250 ok")
		case "MAIL":
			reply(s.mail)
		case "RCPT":
			reply(s.rcpt)
		case "DATA":
			reply(s.data)
			if strings.HasPrefix(s.data, "354") {
				readData(r)
				reply(s.final)
			}
		default:
			reply("221 bye")

			return
		}
	}
}

func readData(r *bufio.Reader) {
	for {
		line, err := r.ReadString('\n')
		if err != nil || line == ".\r\n" {
			return
		}
	}
}

func accepting() script {
	return script{mail: "250 ok", rcpt: "250 ok", data: "354 go ahead", final: "250 2.0.0 Ok"}
}

func TestReplies(t *testing.T) {
	t.Parallel()

	refusals := map[string]struct {
		change func(s *script)
		kind   error
	}{
		"refuses for good when the sender is rejected": {
			func(s *script) { s.mail = "550 5.7.1 Sender rejected" }, mail.ErrPermanent,
		},
		"tries later when a recipient is busy": {
			func(s *script) { s.rcpt = "450 4.2.1 Busy" }, mail.ErrTemporary,
		},
		"refuses for good when data is refused": {
			func(s *script) { s.data = "554 5.3.4 No" }, mail.ErrPermanent,
		},
		"refuses for good when the content is refused": {
			func(s *script) { s.final = "552 5.3.4 Too big" }, mail.ErrPermanent,
		},
	}

	for name, refusal := range refusals {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s := accepting()
			refusal.change(&s)
			mailer, err := smtp.New(clock.System(), serve(t, s))
			require.NoError(t, err)

			_, err = mailer.Send(t.Context(), drivertest.Message(drivertest.Address("Ana")))

			require.ErrorIs(t, err, refusal.kind)
		})
	}

	t.Run("slows down when a recipient is rate limited", func(t *testing.T) {
		t.Parallel()
		s := accepting()
		s.rcpt = "421 4.7.0 Too many connections"
		mailer, err := smtp.New(clock.System(), serve(t, s))
		require.NoError(t, err)

		_, err = mailer.Send(t.Context(), drivertest.Message(drivertest.Address("Ana")))

		var limited *mail.RateLimitedError
		require.ErrorAs(t, err, &limited)
	})

	t.Run("reports no provider id when the reply names no queue", func(t *testing.T) {
		t.Parallel()
		mailer, err := smtp.New(clock.System(), serve(t, accepting()))
		require.NoError(t, err)

		result, err := mailer.Send(t.Context(), drivertest.Message(drivertest.Address("Ana")))

		require.NoError(t, err)
		assert.Empty(t, result.ProviderMessageID)
	})
}

func TestEnvelope(t *testing.T) {
	t.Parallel()

	mailer, err := smtp.New(clock.System(), smtp.Options{Host: "127.0.0.1", Port: 1})
	require.NoError(t, err)

	t.Run("refuses a raw message with a broken sender", func(t *testing.T) {
		t.Parallel()

		_, err := mailer.Send(t.Context(), mail.Message{Raw: []byte("From: <<x\r\nTo: a@example.com\r\n\r\nbody")})

		require.ErrorIs(t, err, mail.ErrPermanent)
	})

	t.Run("refuses a raw message with broken recipients", func(t *testing.T) {
		t.Parallel()

		_, err := mailer.Send(t.Context(), mail.Message{Raw: []byte("From: a@example.com\r\nTo: <<x\r\n\r\nbody")})

		require.ErrorIs(t, err, mail.ErrPermanent)
	})

	t.Run("reports a key that cannot sign", func(t *testing.T) {
		t.Parallel()
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)
		der, err := x509.MarshalPKCS8PrivateKey(key)
		require.NoError(t, err)
		signing, err := smtp.New(clock.System(), smtp.Options{
			Host: "127.0.0.1", Port: 1, DKIMDomain: "example.com", DKIMSelector: "s",
			DKIMPrivateKey: string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})),
		})
		require.NoError(t, err)

		_, err = signing.Send(t.Context(), drivertest.Message(drivertest.Address("Ana")))

		require.Error(t, err)
		require.NotErrorIs(t, err, mail.ErrTemporary)
	})
}
