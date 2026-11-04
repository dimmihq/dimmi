package parse_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/mailbox/parse"
)

func textPart(contentType, body string) []byte {
	header := "From: ana@sender.example\r\nContent-Transfer-Encoding: 8bit\r\n"
	if contentType != "" {
		header += "Content-Type: " + contentType + "\r\n"
	}

	return []byte(header + "\r\n" + body)
}

func TestCharsets(t *testing.T) {
	t.Parallel()

	t.Run("reads labels found in real mail that no registry lists", func(t *testing.T) {
		t.Parallel()

		for _, c := range []struct{ label, bytes, want string }{
			{"iso-gb2312", "\xc4\xe3\xba\xc3", "你好"},
			{"x-big5", "\xa7\x41\xa6\x6e", "你好"},
			{"cp950", "\xa7\x41\xa6\x6e", "你好"},
			{"ms932", "\x82\xb1\x82\xf1", "こん"},
			{"windows-932", "\x82\xb1\x82\xf1", "こん"},
			{"iso-2022-jp-2", "\x1b$B$3$s\x1b(B", "こん"},
			{"x-windows-874", "\xca\xc7\xd1\xca", "สวัส"},
			{"win-1252", "caf\xe9", "café"},
			{"unicode-1-1-utf-7", "+AOk-", "é"},
			{"unknown-8bit", "caf\xe9", "café"},
			{"binary", "caf\xe9", "café"},
			{"charset=utf-8", "café", "café"},
			{"1252", "caf\xe9", "café"},
			{"utf8mb4", "café", "café"},
			{"latin-1-nonsense", "caf\xe9", "café"},
			{`bad"label`, "caf\xe9", "café"},
		} {
			got := parsed(t, textPart(`text/plain; charset="`+strings.ReplaceAll(c.label, `"`, `\"`)+`"`, c.bytes),
				parse.Limits{})
			assert.Equal(t, c.want, strings.TrimSpace(got.Text), c.label)
		}
	})

	t.Run("reads 7-bit Korean, shifted or not", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, textPart("text/plain; charset=iso-2022-kr", "\x1b$)C\x0e>H3g\x0f ok"), parse.Limits{})
		assert.Equal(t, "안녕 ok", got.Text)

		got = parsed(t, textPart("text/plain; charset=iso-2022-kr", "\xbe\xc8\xb3\xe7"), parse.Limits{})
		assert.Equal(t, "안녕", got.Text)

		got = parsed(t, textPart("text/plain", "\x1b$)C\x0e>H3g\x0f"), parse.Limits{})
		assert.Equal(t, "안녕", got.Text)
	})

	t.Run("widens narrow Chinese labels to what senders really write", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, textPart("text/plain; charset=gb2312", "\x81\x30\x81\x30"), parse.Limits{})

		assert.NotContains(t, got.Text, "�")
	})

	t.Run("recognizes Japanese escape sequences with no label", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, textPart("text/plain;", "\x1b$B$3$s\x1b(B"), parse.Limits{})

		assert.Equal(t, "こん", got.Text)
	})

	t.Run("drops a byte order mark", func(t *testing.T) {
		t.Parallel()

		for name, raw := range map[string]string{
			"utf-16": "\xff\xfe\xe9\x00", "utf-16be": "\xfe\xff\x00\xe9", "utf-8": "\xef\xbb\xbfé",
		} {
			got := parsed(t, textPart("text/plain; charset="+name, raw), parse.Limits{})
			assert.Equal(t, "é", got.Text, name)
		}
	})

	t.Run("reads Latin-1 bytes declared as UTF-8 or undeclared", func(t *testing.T) {
		t.Parallel()

		for _, contentType := range []string{"text/plain; charset=utf-8", "text/plain", ""} {
			got := parsed(t, textPart(contentType, "caf\xe9 au lait"), parse.Limits{})
			assert.Equal(t, "café au lait", got.Text, contentType)
		}
	})

	t.Run("keeps good UTF-8 next to a stray byte", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, textPart("text/plain; charset=utf-8", "你好 caf\xe9"), parse.Limits{})

		assert.Equal(t, "你好 café", got.Text)
	})

	t.Run("reads UTF-8 bytes declared as a single-byte charset", func(t *testing.T) {
		t.Parallel()

		for _, label := range []string{"iso-8859-1", "windows-1251", "koi8-r"} {
			got := parsed(t, textPart("text/plain; charset="+label, "café"), parse.Limits{})
			assert.Equal(t, "café", got.Text, label)
		}
	})

	t.Run("trusts a multi-byte label over bytes that look like UTF-8 by accident", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, textPart("text/plain; charset=gb2312", "\xc3\xa9"), parse.Limits{})

		assert.Equal(t, "茅", got.Text)
	})

	t.Run("reads mostly UTF-8 bytes that fail in a declared multi-byte charset", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, textPart("text/plain; charset=gb2312", "äöü - 差出人 - \xe4\xf6\xfc"), parse.Limits{})

		assert.Equal(t, "äöü - 差出人 - äöü", got.Text)
	})

	t.Run("keeps a declared charset that holds", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, textPart("text/plain; charset=koi8-r", "\xf0\xd2\xc9\xd7\xc5\xd4"), parse.Limits{})
		assert.Equal(t, "Привет", got.Text)

		got = parsed(t, textPart("text/plain; charset=gb2312", "\xc4\xe3\xba\xc3"), parse.Limits{})
		assert.Equal(t, "你好", got.Text)
	})

	t.Run("finds the charset named inside html that declares none", func(t *testing.T) {
		t.Parallel()

		body := `<meta charset="koi8-r"><p>` + "\xf0\xd2\xc9\xd7\xc5\xd4</p>"
		got := parsed(t, textPart("text/html", body), parse.Limits{})

		assert.Contains(t, got.HTML, "Привет")
	})

	t.Run("still guesses the charset of long text that declares none", func(t *testing.T) {
		t.Parallel()

		body := strings.Repeat("\xf0\xd2\xc9\xd7\xc5\xd4 \xcd\xc9\xd2 ", 30)
		got := parsed(t, textPart("text/plain", body), parse.Limits{})

		assert.NotContains(t, got.Text, "�")
	})

	t.Run("keeps a text file in UTF-8 and any other file untouched", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, []byte("From: ana@sender.example\r\nContent-Type: multipart/mixed; boundary=m\r\n\r\n"+
			"--m\r\nContent-Type: text/csv; charset=iso-8859-1; name=a.csv\r\nContent-Disposition: attachment\r\n\r\n"+
			"caf\xe9\r\n--m\r\nContent-Type: application/x-thing; name=b.bin\r\n\r\ncaf\xe9\r\n--m--\r\n"),
			parse.Limits{})

		assert.Equal(t, []byte("café"), got.Attachments[0].Content)
		assert.Equal(t, []byte("caf\xe9"), got.Attachments[1].Content)
	})
}

func TestEncodedWords(t *testing.T) {
	t.Parallel()

	for name, c := range map[string]struct{ header, want string }{
		"excess padding":             {"=?ISO-2022-JP?B?GyRCO3ZMMzZJGyhC==?=", "事務局"},
		"base64 split between words": {"=?utf-8?B?5L?= =?utf-8?B?2g5aW9?=", "你好"},
		"character split in Q words": {"=?utf-8?Q?caf=C3?= =?utf-8?Q?=A9?=", "café"},
		"padded words":               {"=?utf-8?B?5L2g?= =?utf-8?B?5aW9?=", "你好"},
		"unknown charset":            {"=?unknown-8bit?Q?caf=E9?=", "café"},
		"unregistered utf-7 label":   {"=?unicode-1-1-utf-7?Q?+AOk-?=", "é"},
		"spaces inside a word":       {"=?us-ascii?Q?one / two?=", "one / two"},
		"language suffix":            {"=?utf-8*en?Q?hello?=", "hello"},
		"text around words":          {"Re: =?utf-8?Q?caf=C3=A9?= again", "Re: café again"},
		"different charsets":         {"=?iso-8859-1?Q?caf=E9?= =?utf-8?Q?=C3=A9?=", "caféé"},
		"mixed encodings":            {"=?utf-8?B?w6k=?= =?utf-8?Q?=C3=A9?=", "éé"},
		"broken hex stays":           {"=?utf-8?Q?50=ZZ=?=", "50=ZZ="},
		"junk in base64":             {"=?utf-8?B?5L2g!5aW9?=", "你好"},
		"cut base64":                 {"=?utf-8?B?5L2gA?=", "你"},
		"not a word":                 {"=?utf-8?B?5L2g5aW9?", "=?utf-8?B?5L2g5aW9?"},
	} {
		t.Run("reads a subject with "+name, func(t *testing.T) {
			t.Parallel()

			got := parsed(t, []byte("From: ana@sender.example\r\nSubject: "+c.header+"\r\n\r\nx"), parse.Limits{})

			assert.Equal(t, c.want, got.Subject)
		})
	}

	t.Run("reads a sender name with excess padding", func(t *testing.T) {
		t.Parallel()

		raw := "From: =?ISO-2022-JP?B?GyRCO3ZMMzZJGyhC==?= <ana@sender.example>\r\n\r\nx"
		got := parsed(t, []byte(raw), parse.Limits{})

		assert.Equal(t, "事務局", got.From.Name)
		assert.Equal(t, "ana@sender.example", got.From.Email)
	})

	t.Run("reads a file name with excess padding", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, []byte("From: ana@sender.example\r\nContent-Type: multipart/mixed; boundary=m\r\n\r\n"+
			"--m\r\nContent-Type: application/pdf\r\nContent-Disposition: attachment; "+
			"filename=\"=?ISO-2022-JP?B?GyRCO3ZMMzZJGyhC==?=.pdf\"\r\n\r\nx\r\n--m--\r\n"), parse.Limits{})

		assert.Equal(t, "事務局.pdf", got.Attachments[0].Filename)
	})
}
