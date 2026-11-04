package dom_test

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/dom"
)

/*
failingReader fails every read, like a connection that drops.
*/
type failingReader struct{}

/*
Read always fails.
*/
func (failingReader) Read([]byte) (int, error) {
	return 0, errors.New("connection reset")
}

/*
fixture reads a file from testdata.
*/
func fixture(t *testing.T, name string) string {
	t.Helper()

	b, err := fs.ReadFile(os.DirFS("testdata"), name)
	require.NoError(t, err)

	return string(b)
}

/*
deeplyNested returns markup nested deeper than any parser accepts.
*/
func deeplyNested() string {
	return strings.Repeat("<div>", 600) + "x"
}

func TestDocument(t *testing.T) {
	t.Parallel()

	t.Run("finds, reads and changes elements", func(t *testing.T) {
		t.Parallel()

		doc, err := dom.Parse(strings.NewReader(fixture(t, "email.html")))
		require.NoError(t, err)

		pixel := doc.Find(`img[width="1"]`)
		src, ok := pixel.Attr("src")
		assert.True(t, ok)
		assert.Equal(t, "https://track.example/p.gif", src)

		_, ok = pixel.Attr("alt")
		assert.False(t, ok)

		assert.Equal(t, 1, doc.Find("div.gmail_quote").Remove().Len())
		assert.Equal(t, 1, pixel.Remove().Len())
		doc.Find("div").SetAttr("dir", "rtl")

		body := doc.Find("body")
		assert.Equal(t, "Thanks, that fixed it!", strings.TrimSpace(body.Text()))
		assert.Equal(t, "<div dir=\"rtl\">Thanks, that fixed it!<br/></div>\n<br/>", strings.TrimSpace(body.HTML()))
		assert.Contains(t, doc.HTML(), "<html><head></head><body>")
	})

	t.Run("walks selections", func(t *testing.T) {
		t.Parallel()

		doc, err := dom.Parse(strings.NewReader(`<ul><li>a</li><li id="b">b</li><li>c</li><li>d</li></ul>`))
		require.NoError(t, err)

		var items []string
		doc.Find("ul").Find("li").Each(func(s dom.Selection) {
			items = append(items, s.Text())
		})

		assert.Equal(t, []string{"a", "b", "c", "d"}, items)
		assert.Equal(t, "cd", doc.Find("#b").NextAll().Text())
	})

	t.Run("matches nothing for a broken selector", func(t *testing.T) {
		t.Parallel()

		doc, err := dom.Parse(strings.NewReader(`<p>x</p>`))
		require.NoError(t, err)

		sel := doc.Find("p[")

		assert.Zero(t, sel.Len())
		assert.Empty(t, sel.Text())
		assert.Empty(t, sel.HTML())
		assert.Zero(t, sel.Remove().Len())
	})

	t.Run("repairs malformed markup like a browser", func(t *testing.T) {
		t.Parallel()

		doc, err := dom.Parse(strings.NewReader(`<p>one<p>two<b>bold</p><table><td>cell`))
		require.NoError(t, err)

		assert.Equal(t, 2, doc.Find("p").Len())
		assert.Equal(t, "cell", doc.Find("table tr td").Text())
	})

	t.Run("refuses markup nested too deep", func(t *testing.T) {
		t.Parallel()

		_, err := dom.Parse(strings.NewReader(deeplyNested()))

		require.Error(t, err)
	})

	t.Run("reports a failing reader", func(t *testing.T) {
		t.Parallel()

		_, err := dom.Parse(failingReader{})

		require.ErrorContains(t, err, "connection reset")
	})
}

func TestConvert(t *testing.T) {
	t.Parallel()

	t.Run("turns html into text", func(t *testing.T) {
		t.Parallel()

		got := dom.ToText(`<p>Fish &amp; <b>chips</b></p><ul><li>one</li><li>two</li></ul>` +
			`<a href="https://example.com">site</a><script>alert(1)</script><style>p{}</style>`)

		assert.Equal(t, "Fish & chips\n\n\n - one\n - two\nsite <https://example.com>", got)
	})

	t.Run("turns html into markdown", func(t *testing.T) {
		t.Parallel()

		got, err := dom.ToMarkdown(`<h2>Title</h2><p>Hi <b>there</b> <a href="/x">link</a></p>` +
			`<ul><li>one</li></ul><script>alert(1)</script><style>p{}</style><iframe src="x"></iframe>`)
		require.NoError(t, err)

		assert.Equal(t, "## Title\n\nHi **there** [link](/x)\n\n- one", got)
	})

	t.Run("refuses markup nested too deep", func(t *testing.T) {
		t.Parallel()

		_, err := dom.ToMarkdown(deeplyNested())

		require.Error(t, err)
	})
}

func TestReadable(t *testing.T) {
	t.Parallel()

	t.Run("extracts the main content of a page", func(t *testing.T) {
		t.Parallel()

		got, err := dom.Readable(fixture(t, "page.html"), "https://help.example.com/guides/reset")
		require.NoError(t, err)

		assert.Equal(t, "Resetting your password | Example Help", got.Title)
		assert.Equal(t, "Ada Lovelace", got.Byline)
		assert.Equal(t, "Example Help", got.SiteName)
		assert.Equal(t, "en", got.Language)
		assert.True(t, strings.HasPrefix(got.Excerpt, "If you forgot your password"))
		assert.Contains(t, got.HTML, `href="https://help.example.com/support"`)
		assert.Contains(t, got.HTML, `src="https://help.example.com/images/reset.png"`)
		assert.Contains(t, got.Text, "email support and we will help.")

		for _, noise := range []string{"Pricing", "cookies", "Copyright", "tracking", "alert", "color: red"} {
			assert.NotContains(t, got.HTML, noise)
			assert.NotContains(t, got.Text, noise)
		}
	})

	t.Run("reports a page without content", func(t *testing.T) {
		t.Parallel()

		_, err := dom.Readable("<html><body></body></html>", "https://example.com")

		require.ErrorIs(t, err, dom.ErrNoContent)
	})

	t.Run("rejects a page url that does not parse", func(t *testing.T) {
		t.Parallel()

		_, err := dom.Readable("<p>x</p>", "%zz")

		require.ErrorContains(t, err, "parse page url")
	})

	t.Run("refuses markup nested too deep", func(t *testing.T) {
		t.Parallel()

		_, err := dom.Readable(deeplyNested(), "https://example.com")

		require.ErrorContains(t, err, "extract readable content")
	})
}

func TestDecodeCharset(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name, in, declared, want string
	}{
		{"declared charset wins", "caf\xe9", "iso-8859-1", "café"},
		{"declared label alias", "caf\xe9", "latin1", "café"},
		{"valid utf-8 without a label", "caf\xc3\xa9", "", "café"},
		{"unknown label counts as none", "caf\xc3\xa9", "x-bogus", "café"},
		{"meta tag names the charset", "<meta charset=shift_jis>\x93\xfa\x96\x7b", "", "<meta charset=shift_jis>日本"},
		{"windows-1252 when nothing fits", "\x93quoted\x94", "", "“quoted”"},
		{"byte order mark beats the label", "\xff\xfeA\x00", "iso-8859-1", "\ufeffA"},
		{"invalid bytes in declared utf-8", "a\xffb", "utf-8", "a�b"},
		{"plain ascii", "hello", "", "hello"},
		{"empty input", "", "", ""},
	}

	for _, tc := range cases {
		got, err := dom.DecodeCharset([]byte(tc.in), tc.declared)
		require.NoError(t, err, tc.name)

		assert.Equal(t, tc.want, got, tc.name)
	}
}

func TestStripTags(t *testing.T) {
	t.Parallel()

	t.Run("reads a document nested too deep to parse", func(t *testing.T) {
		t.Parallel()

		document := strings.Repeat("<div>", 600) + "Deep &amp; true" + strings.Repeat("</div>", 600)

		_, err := dom.Parse(strings.NewReader(document))
		require.Error(t, err)
		assert.Equal(t, "Deep & true", dom.StripTags(document, nil))
	})

	t.Run("leaves out scripts, styles and the head and puts blocks on their own lines", func(t *testing.T) {
		t.Parallel()

		got := dom.StripTags("<head><title>No</title></head><style>p{}</style><script>x()</script>"+
			"<p>One <b>bold</b></p><!-- no --><p>Two<br/>Three</p><img src=x>Four", nil)

		assert.Equal(t, "One bold\n\nTwo\nThree\nFour", got)
	})

	t.Run("leaves out what the caller picks, with everything inside it", func(t *testing.T) {
		t.Parallel()

		document := `<div class="a">Keep</div><div class="no">Drop<div>Drop too</div>Drop more</div><p>End</p>`
		got := dom.StripTags(document,
			func(tag dom.Tag) bool { return tag.Attrs["class"] == "no" })

		assert.Equal(t, "Keep\n\nEnd", got)
	})

	t.Run("does not start skipping at an element that holds nothing", func(t *testing.T) {
		t.Parallel()

		got := dom.StripTags(`<img class="no">Keep`, func(tag dom.Tag) bool { return tag.Attrs["class"] == "no" })

		assert.Equal(t, "Keep", got)
	})
}
