package parse_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/mailbox/parse"
)

func TestDates(t *testing.T) {
	t.Parallel()

	for date, want := range map[string]string{
		"Wed, 5 Sep 2018 10:00:00 +0200":                           "2018-09-05T08:00:00Z",
		"Wed, 5 Sep 2018 10:00:00 +0200 (W. Europe Daylight Time)": "2018-09-05T08:00:00Z",
		"Wed, 5 Sep 2018 10:00:00 GMT":                             "2018-09-05T10:00:00Z",
		"Wed, 5 Sep 2018 10:00:00 UT":                              "2018-09-05T10:00:00Z",
		"Wed, 5 Sep 2018 10:00:00 EST":                             "2018-09-05T15:00:00Z",
		"Wed, 5 Sep 2018 10:00:00 pdt":                             "2018-09-05T17:00:00Z",
		"Wed, 5 Sep 2018 10:00:00 CST":                             "2018-09-05T16:00:00Z",
		"Wed, 5 Sep 2018 10:00:00 MDT":                             "2018-09-05T16:00:00Z",
		"Wed, 5 Sep 2018 10:00:00 EDT":                             "2018-09-05T14:00:00Z",
		"Wed, 5 Sep 2018 10:00:00 PST":                             "2018-09-05T18:00:00Z",
	} {
		t.Run("reads the zone of "+date, func(t *testing.T) {
			t.Parallel()

			got := parsed(t, []byte("From: a@x.example\r\nDate: "+date+"\r\n\r\nx"), parse.Limits{})

			assert.Equal(t, want, got.Date.UTC().Format("2006-01-02T15:04:05Z"))
		})
	}

	for _, date := range []string{
		"Wed, 5 Sep 2018 10:00:00 CET", "Wed, 5 Sep 2018 10:00:00 IST", "Wed, 5 Sep 2018 10:00:00 CEST",
		"Wed, 5 Sep 2018 29:00:00 +0200", "garbage",
	} {
		t.Run("counts "+date+" as unreadable", func(t *testing.T) {
			t.Parallel()

			got := parsed(t, []byte("From: a@x.example\r\nDate: "+date+"\r\n\r\nx"), parse.Limits{})

			assert.True(t, got.Date.IsZero())
			assert.Contains(t, got.Warnings, "unreadable date: "+date)
		})
	}
}

func TestAttachmentNames(t *testing.T) {
	t.Parallel()

	attached := func(disposition, contentType, encoding, body string) []byte {
		return []byte("From: a@x.example\r\nContent-Type: multipart/mixed; boundary=m\r\n\r\n--m\r\n" +
			"Content-Type: text/plain\r\n\r\nBody\r\n--m\r\nContent-Type: " + contentType + "\r\n" +
			"Content-Disposition: " + disposition + "\r\nContent-Transfer-Encoding: " + encoding + "\r\n\r\n" +
			body + "\r\n--m--\r\n")
	}

	for name, want := range map[string]string{
		`INVOICE.EXE `:       "INVOICE.EXE",
		`invoice.exe.`:       "invoice.exe",
		`invoice.exe . .`:    "invoice.exe",
		`../../etc/passwd`:   "passwd",
		`C:\\temp\\evil.pdf`: "evil.pdf",
		`..`:                 "",
		"bad\u202Egnp.exe":   "badgnp.exe",
		`.hidden`:            ".hidden",
		`a.b.pdf`:            "a.b.pdf",
	} {
		t.Run("stores "+name+" under a safe name", func(t *testing.T) {
			t.Parallel()

			got := parsed(t, attached(`attachment; filename="`+name+`"`, "application/octet-stream", "7bit", "x"),
				parse.Limits{})

			require.Len(t, got.Attachments, 1)
			assert.Equal(t, want, got.Attachments[0].Filename)
		})
	}

	t.Run("names an attached mail after its subject", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, attached("attachment", "message/rfc822", "7bit",
			"From: b@x.example\r\nSubject: =?utf-8?Q?Caf=C3=A9/plan?=\r\n\r\nInner"), parse.Limits{})

		require.Len(t, got.Attachments, 1)
		assert.Equal(t, "plan.eml", got.Attachments[0].Filename)
	})

	t.Run("leaves an attached mail without a subject unnamed", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, attached("attachment", "message/rfc822", "7bit", "From: b@x.example\r\n\r\nInner"),
			parse.Limits{})

		require.Len(t, got.Attachments, 1)
		assert.Empty(t, got.Attachments[0].Filename)
	})

	uuencoded := "begin 644 hello.txt\r\n%:&5L;&\\`\r\n`\r\nend"

	t.Run("decodes a uuencoded attachment and takes its name", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, attached("attachment", "application/octet-stream", "x-uuencode", uuencoded), parse.Limits{})

		require.Len(t, got.Attachments, 1)
		assert.Equal(t, []byte("hello"), got.Attachments[0].Content)
		assert.Equal(t, int64(5), got.Attachments[0].Size)
		assert.Equal(t, "hello.txt", got.Attachments[0].Filename)
	})

	t.Run("decodes uuencoded lines sent without a begin line", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, attached("attachment", "application/octet-stream", "x-uuencode", "%:&5L;&\\`\r\n`"),
			parse.Limits{})

		require.Len(t, got.Attachments, 1)
		assert.Equal(t, []byte("hello"), got.Attachments[0].Content)
	})

	t.Run("prefers the name a uuencoded attachment declares", func(t *testing.T) {
		t.Parallel()

		got := parsed(t,
			attached(`attachment; filename="given.txt"`, "application/octet-stream", "uuencode", uuencoded),
			parse.Limits{})

		require.Len(t, got.Attachments, 1)
		assert.Equal(t, "given.txt", got.Attachments[0].Filename)
	})

	t.Run("keeps content as it came when the uuencoding promise is broken", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, attached("attachment", "application/octet-stream", "x-uue", "not encoded"), parse.Limits{})

		require.Len(t, got.Attachments, 1)
		assert.Equal(t, []byte("not encoded"), got.Attachments[0].Content)
	})

	t.Run("decodes a uuencoded text body", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, []byte("From: a@x.example\r\nContent-Type: text/plain\r\n"+
			"Content-Transfer-Encoding: x-uuencode\r\n\r\n"+uuencoded), parse.Limits{})

		assert.Equal(t, "hello", got.Text)
	})

	t.Run("decodes a line a mailer stripped trailing spaces from", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, attached("attachment", "application/octet-stream", "x-uuencode",
			"begin 644 a\r\n!80\r\n\r\nend"), parse.Limits{})

		assert.Equal(t, []byte("a"), got.Attachments[0].Content)
	})
}
