package parse_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/mailbox/parse"
)

func TestSloppyAddresses(t *testing.T) {
	t.Parallel()

	for name, c := range map[string]struct {
		from string
		want mail.Address
	}{
		"an unquoted comma in the name": {
			"Mueller, Hans <hans@x.example>", mail.Address{Name: "Mueller, Hans", Email: "hans@x.example"},
		},
		"an encoded comma in the name": {
			"=?iso-8859-1?Q?B=FCrling,=20Andreas?= <a@x.example>",
			mail.Address{Name: "Bürling, Andreas", Email: "a@x.example"},
		},
		"brackets in the name": {
			"Foo [Bar] <foo@x.example>", mail.Address{Name: "Foo [Bar]", Email: "foo@x.example"},
		},
		"a colon in the name": {
			"Foo: Bar <foo@x.example>", mail.Address{Name: "Foo: Bar", Email: "foo@x.example"},
		},
		"an at sign in the name": {
			"Foo @ Bar <foo@x.example>", mail.Address{Name: "Foo @ Bar", Email: "foo@x.example"},
		},
		"a trailing dot on the domain": {
			"Bob <bob@x.example.>", mail.Address{Name: "Bob", Email: "bob@x.example"},
		},
		"an empty address before a good one": {
			`"A" <>, "B" <b@x.example>`, mail.Address{Name: "B", Email: "b@x.example"},
		},
		"only a quoted address": {
			`"hans@x.example"`, mail.Address{Email: "hans@x.example"},
		},
		"no brackets around the address": {
			"John Doe john@x.example", mail.Address{Name: "John Doe", Email: "john@x.example"},
		},
		"a mailto address": {
			"Ana: L <mailto:ana@x.example>", mail.Address{Name: "Ana: L", Email: "ana@x.example"},
		},
		"brackets never closed": {
			"Ana, L <ana@x.example", mail.Address{Name: "Ana, L", Email: "ana@x.example"},
		},
		"a quote never closed": {
			`"Ana, L <ana@x.example>`, mail.Address{Name: "Ana, L", Email: "ana@x.example"},
		},
		"only a name": {
			"Bob Lima", mail.Address{Name: "Bob Lima"},
		},
		"a name beside a broken address": {
			`"Bob Lima" <bob@one@x.example>`, mail.Address{Name: "Bob Lima"},
		},
		"nothing readable": {
			"<>", mail.Address{},
		},
		"only punctuation": {
			"[?????]", mail.Address{},
		},
	} {
		t.Run("reads a sender with "+name, func(t *testing.T) {
			t.Parallel()

			got := parsed(t, []byte("From: "+c.from+"\r\nSubject: s\r\n\r\nx"), parse.Limits{})

			assert.Equal(t, c.want, got.From)
		})
	}

	t.Run("reads the second From field when the first has no address", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, []byte("From: [?????]\r\nFrom: Real <real@x.example>\r\n\r\nx"), parse.Limits{})

		assert.Equal(t, mail.Address{Name: "Real", Email: "real@x.example"}, got.From)
	})

	t.Run("keeps to the first From field that has an address", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, []byte("From: first@x.example\r\nFrom: second@x.example\r\n\r\nx"), parse.Limits{})

		assert.Equal(t, "first@x.example", got.From.Email)
	})

	t.Run("falls back to the Sender field and keeps the name in From", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, []byte("From: Bob Lima\r\nSender: Relay <relay@x.example>\r\n\r\nx"), parse.Limits{})

		assert.Equal(t, mail.Address{Name: "Bob Lima", Email: "relay@x.example"}, got.From)
	})

	t.Run("reads every address of a recipient list that breaks the rules", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, []byte("From: a@x.example\r\nTo: Lima, Ana <ana@x.example>, bo@x.example,"+
			" [Sales] <sales@x.example>\r\nCc: one@x.example\r\nCc: two@x.example\r\n\r\nx"),
			parse.Limits{})

		require.Len(t, got.To, 3)
		assert.Equal(t, mail.Address{Name: "Lima, Ana", Email: "ana@x.example"}, got.To[0])
		assert.Equal(t, mail.Address{Email: "bo@x.example"}, got.To[1])
		assert.Equal(t, mail.Address{Name: "[Sales]", Email: "sales@x.example"}, got.To[2])
		assert.Len(t, got.Cc, 2)
	})

	t.Run("reads raw bytes in a name in the charset of the body", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, []byte("From: \xc4\xe3\xba\xc3 <a@x.example>\r\nSubject: \xc4\xe3\xba\xc3\r\n"+
			"Content-Type: text/plain; charset=gb2312\r\n\r\nx"), parse.Limits{})

		assert.Equal(t, "你好", got.Subject)
		assert.Equal(t, "你好", got.From.Name)
	})

	t.Run("reads raw Shift_JIS in a header of mail that declares 7-bit Japanese", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, []byte("From: a@x.example\r\nSubject: \x83\x63\x81\x5b\x83\x8b\r\n"+
			"Content-Type: text/plain; charset=iso-2022-jp\r\n\r\nx"), parse.Limits{})

		assert.Equal(t, "ツール", got.Subject)
	})

	t.Run("reads raw bytes in a header of mail that declares nothing useful", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, []byte("From: a@x.example\r\nSubject: caf\xe9\r\n"+
			"Content-Type: text/plain; charset=utf-8\r\n\r\nx"), parse.Limits{})

		assert.Equal(t, "café", got.Subject)
	})
}

func TestBrokenHeaders(t *testing.T) {
	t.Parallel()

	for name, raw := range map[string]string{
		"a leading blank line":      "\r\nFrom: a@x.example\r\nSubject: s\r\n\r\nbody",
		"a junk first line":         "---- Original email message ----\r\nFrom: a@x.example\r\nSubject: s\r\n\r\nbody",
		"an mbox separator":         "From a@x.example  Wed May 20 2026\r\nFrom: a@x.example\r\nSubject: s\r\n\r\nbody",
		"a stray line after From":   "From: a@x.example\r\nGarbage without a colon\r\nSubject: s\r\n\r\nbody",
		"bare line feeds":           "From: a@x.example\nSubject: s\n\nbody",
		"a boundary-less multipart": "From: a@x.example\r\nSubject: s\r\nContent-Type: multipart/mixed\r\n\r\nbody",
		"an unusable content type":  "From: a@x.example\r\nSubject: s\r\nContent-Type: ;;;/\r\n\r\nbody",
	} {
		t.Run("reads sender, subject and body with "+name, func(t *testing.T) {
			t.Parallel()

			got := parsed(t, []byte(raw), parse.Limits{})

			assert.Equal(t, "a@x.example", got.From.Email)
			assert.Equal(t, "s", got.Subject)
			assert.Equal(t, "body", got.Text)
		})
	}

	t.Run("reads a boundary that matches nothing as raw text", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, []byte("From: a@x.example\r\nContent-Type: multipart/mixed; boundary=zzz\r\n\r\n"+
			"--yyy\r\nContent-Type: text/plain\r\n\r\nbody text\r\n--yyy--"), parse.Limits{})

		assert.Equal(t, "a@x.example", got.From.Email)
		assert.Contains(t, got.Text, "body text")
		assert.NotEmpty(t, got.Warnings)
	})

	t.Run("undoes the transfer encoding of a body read as raw text", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, []byte("From: a@x.example\r\nContent-Transfer-Encoding: quoted-printable\r\n"+
			"Content-Type: multipart/mixed; boundary=zzz\r\n\r\ncaf=C3=A9"), parse.Limits{})

		assert.Equal(t, "café", got.Text)
	})

	t.Run("keeps the words of a nested part whose boundary matches nothing", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, []byte("From: a@x.example\r\nContent-Type: multipart/mixed; boundary=m\r\n\r\n"+
			"--m\r\nContent-Type: multipart/alternative; boundary=zzz\r\n\r\nNested words\r\n--m\r\n"+
			"Content-Type: application/pdf; name=a.pdf\r\n\r\nx\r\n--m--\r\n"), parse.Limits{})

		assert.Contains(t, got.Text, "Nested words")
		assert.Len(t, got.Attachments, 1)
	})

	t.Run("joins folded fields without doubling spaces", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, []byte("From: a@x.example\r\nSubject: one  \r\n\ttwo\r\n three\r\n\r\nx"), parse.Limits{})

		assert.Equal(t, "one two three", got.Subject)
	})

	t.Run("reads a header with no body under it", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, []byte("From: a@x.example\r\nSubject: s"), parse.Limits{})

		assert.Equal(t, "a@x.example", got.From.Email)
		assert.Equal(t, "s", got.Subject)
		assert.Empty(t, got.Text)
	})
}

func TestOriginalSender(t *testing.T) {
	t.Parallel()

	t.Run("reads the author a group names when it sends mail on", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, []byte("From: 'Jane Roe' via Support <support@acme.example>\r\n"+
			"X-Original-Sender: jane@customer.example\r\n"+
			"X-Original-From: Jane Roe <jane@customer.example>\r\nSubject: s\r\n\r\nbody"), parse.Limits{})

		assert.Equal(t, mail.Address{Name: "Jane Roe", Email: "jane@customer.example"}, got.OriginalSender)
	})

	t.Run("falls back to the original sender field", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, []byte("From: support@acme.example\r\nX-Original-Sender: jane@customer.example\r\n"+
			"Subject: s\r\n\r\nbody"), parse.Limits{})

		assert.Equal(t, "jane@customer.example", got.OriginalSender.Email)
	})

	t.Run("names nobody when the mail names no original author", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, []byte("From: a@x.example\r\nSubject: s\r\n\r\nbody"), parse.Limits{})

		assert.Empty(t, got.OriginalSender.Email)
	})
}
