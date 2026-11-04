package parse_test

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/mailbox/parse"
)

/*
mime builds a message from header lines and a body, with the line ending
mail uses.
*/
func mime(header, body string) []byte {
	return []byte(strings.ReplaceAll(header+"\n\n"+body, "\n", "\r\n"))
}

const pdfPart = "Content-Type: application/pdf; name=report.pdf\n" +
	"Content-Disposition: inline; filename=report.pdf\nContent-Transfer-Encoding: base64\n\nZGF0YQ==\n"

func TestBodyParts(t *testing.T) {
	t.Parallel()

	header := "From: Ana <ana@sender.example>\nMIME-Version: 1.0\nContent-Type: "

	t.Run("joins text on both sides of a file with a plain paragraph break", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, mime(header+"multipart/mixed; boundary=m",
			"--m\nContent-Type: text/plain\n\nFirst para\n--m\n"+pdfPart+
				"--m\nContent-Type: text/plain\n\nSecond para\n--m--"), parse.Limits{})

		assert.Equal(t, "First para\n\nSecond para", strings.ReplaceAll(strings.TrimSpace(got.Text), "\r\n", "\n"))
		assert.NotContains(t, got.Text, "--")
		require.Len(t, got.Attachments, 1)
		assert.Equal(t, "report.pdf", got.Attachments[0].Filename)
	})

	t.Run("joins html on both sides of a file", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, mime(header+"multipart/alternative; boundary=o",
			"--o\nContent-Type: text/plain\n\nFirst para\n\nSecond para\n--o\n"+
				"Content-Type: multipart/mixed; boundary=m\n\n--m\nContent-Type: text/html\n\n"+
				"<html><body>First para</body></html>\n--m\n"+pdfPart+
				"--m\nContent-Type: text/html\n\n<html><body>Second para</body></html>\n--m--\n--o--"), parse.Limits{})

		assert.Contains(t, got.HTML, "First para")
		assert.Contains(t, got.HTML, "Second para")
		assert.False(t, got.TextDerived)
		require.Len(t, got.Attachments, 1)
	})

	t.Run("keeps a text part that follows an html part in the html too", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, mime(header+"multipart/mixed; boundary=m",
			"--m\nContent-Type: text/html\n\n<p>Ana wrote</p>\n--m\n"+
				"Content-Type: text/plain\n\nA <b> footer\nline two\n--m--"),
			parse.Limits{})

		assert.Contains(t, got.HTML, "<p>Ana wrote</p>")
		assert.Contains(t, got.HTML, "<p>A &lt;b&gt; footer<br>line two</p>")
		assert.Contains(t, got.Text, "Ana wrote")
		assert.Contains(t, got.Text, "A <b> footer")
	})

	t.Run("does not list body parts marked inline as attachments", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, mime(header+"multipart/alternative; boundary=m",
			"--m\nContent-Type: text/plain\nContent-Disposition: inline\n\nThe text\n--m\n"+
				"Content-Type: text/html\nContent-Disposition: inline\n\n<p>The html</p>\n--m--"), parse.Limits{})

		assert.Equal(t, "The text", strings.TrimSpace(got.Text))
		assert.Equal(t, "<p>The html</p>", strings.TrimSpace(got.HTML))
		assert.Empty(t, got.Attachments)
	})

	t.Run("lists a named text file as an attachment, not as the body", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, mime(header+"multipart/mixed; boundary=m",
			"--m\nContent-Type: text/plain\n\nThe text\n--m\n"+
				"Content-Type: text/plain; name=notes.txt\n\nFile contents\n--m--"), parse.Limits{})

		assert.Equal(t, "The text", strings.TrimSpace(got.Text))
		require.Len(t, got.Attachments, 1)
		assert.Equal(t, "notes.txt", got.Attachments[0].Filename)
	})

	t.Run("reads a named text part as the body when there is no other", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, mime(header+"multipart/mixed; boundary=m",
			"--m\nContent-Type: text/plain; name=message.txt\n\nThe only words\n--m--"), parse.Limits{})

		assert.Equal(t, "The only words", strings.TrimSpace(got.Text))
	})

	t.Run("never takes a text attachment for the body", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, mime(header+"multipart/mixed; boundary=m",
			"--m\nContent-Type: text/plain; name=a.txt\n"+
				"Content-Disposition: attachment; filename=a.txt\n\nFile\n--m--"),
			parse.Limits{})

		assert.Empty(t, got.Text)
		require.Len(t, got.Attachments, 1)
	})

	t.Run("lists a single binary message as an attachment", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, mime(
			"From: ana@sender.example\nContent-Type: application/pdf\nContent-Transfer-Encoding: base64",
			base64.StdEncoding.EncodeToString([]byte("data"))), parse.Limits{})

		assert.Empty(t, got.Text)
		require.Len(t, got.Attachments, 1)
		assert.Equal(t, []byte("data"), got.Attachments[0].Content)
	})

	t.Run("marks text converted from html as not the sender's own", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, mime("From: ana@sender.example\nContent-Type: text/html", "<p>Only html</p>"), parse.Limits{})

		assert.Equal(t, "Only html", strings.TrimSpace(got.Text))
		assert.True(t, got.TextDerived)
		assert.Empty(t, got.TextPart())
	})

	t.Run("gives the sender's own text as the text part", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, mime("From: ana@sender.example", "Own words"), parse.Limits{})

		assert.Equal(t, "Own words", got.TextPart())
	})
}
