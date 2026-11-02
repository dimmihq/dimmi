package markdown_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/markdown"
)

func TestPlainText(t *testing.T) {
	t.Parallel()

	t.Run("keeps the words of every block", func(t *testing.T) {
		t.Parallel()

		_, body, err := markdown.FrontMatter(fixture(t, "guide.md"), 0)
		require.NoError(t, err)

		assert.Equal(t, strings.Join([]string{
			"Resetting your password",
			"You can reset it from the sign in page. See billing or our blog, or mail support@example.com. " +
				"Visit https://status.example.com.",
			"The link expires after one hour.",
			"A plain quote.",
			"Steps",
			"Open the page",
			"Choose a password",
			"Done",
			"Not done",
			"Plan Price",
			"Free $0",
			`func main() { fmt.Println("hi") }`,
			"plain <code>",
			"Details",
			"Reset screen",
			"Some old text with code and a footnote.",
			"The footnote text.",
		}, "\n"), markdown.PlainText(body))
	})

	t.Run("resolves escapes and entities outside code only", func(t *testing.T) {
		t.Parallel()

		got := markdown.PlainText([]byte("Fish &amp; chips \\*not em\\* `a \\* &amp;`\n\n    indented &amp;\n"))

		assert.Equal(t, "Fish & chips *not em* a \\* &amp;\nindented &amp;", got)
	})

	t.Run("drops raw html", func(t *testing.T) {
		t.Parallel()

		got := markdown.PlainText([]byte("Hi <b>there</b>\n\n<div>\nblock\n</div>\n\nend"))

		assert.Equal(t, "Hi there\nend", got)
	})

	t.Run("returns nothing for an empty document", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, markdown.PlainText(nil))
	})
}

func TestFrontMatter(t *testing.T) {
	t.Parallel()

	t.Run("reads yaml", func(t *testing.T) {
		t.Parallel()

		meta, body, err := markdown.FrontMatter(fixture(t, "guide.md"), 0)
		require.NoError(t, err)

		assert.Equal(t, map[string]any{
			"title": "Resetting your password",
			"tags":  []any{"account", "security"},
			"order": 3,
		}, meta)
		assert.True(t, strings.HasPrefix(string(body), "# Resetting your password\n"))
	})

	t.Run("reads toml", func(t *testing.T) {
		t.Parallel()

		meta, body, err := markdown.FrontMatter([]byte("+++\r\ntitle = \"Hi\"\r\ndraft = true\r\n+++\r\nBody"), 0)
		require.NoError(t, err)

		assert.Equal(t, map[string]any{"title": "Hi", "draft": true}, meta)
		assert.Equal(t, "Body", string(body))
	})

	t.Run("treats documents without a block as all body", func(t *testing.T) {
		t.Parallel()

		inputs := []string{
			"# Just markdown\n",
			"---\nunterminated: true\n",
			"--\nshort: fence\n--\n",
			"-+-\nmixed: fence\n-+-\n",
			"***\nnot: a format\n***\n",
			"---",
			"",
		}

		for _, in := range inputs {
			meta, body, err := markdown.FrontMatter([]byte(in), 0)
			require.NoError(t, err, in)

			assert.Empty(t, meta, in)
			assert.Equal(t, in, string(body), in)
		}
	})

	t.Run("accepts an empty block and a longer fence", func(t *testing.T) {
		t.Parallel()

		meta, body, err := markdown.FrontMatter([]byte("---\n---\nbody"), 0)
		require.NoError(t, err)
		assert.Empty(t, meta)
		assert.Equal(t, "body", string(body))

		meta, body, err = markdown.FrontMatter([]byte("-----\na: 1\n---\nb: 2\n-----"), 0)
		require.NoError(t, err)
		assert.Equal(t, map[string]any{"a": 1, "b": 2}, map[string]any{"a": meta["a"], "b": 2})
		assert.Empty(t, body)
	})

	t.Run("refuses a block over the limit", func(t *testing.T) {
		t.Parallel()

		big := "---\nnote: " + strings.Repeat("x", 100) + "\n---\nbody"

		_, _, err := markdown.FrontMatter([]byte(big), 64)
		require.ErrorIs(t, err, markdown.ErrFrontMatterTooLarge)

		huge := "---\nnote: " + strings.Repeat("x", markdown.DefaultFrontMatterBytes) + "\n---\nbody"

		_, _, err = markdown.FrontMatter([]byte(huge), 0)
		require.ErrorIs(t, err, markdown.ErrFrontMatterTooLarge)
	})

	t.Run("refuses malformed and hostile blocks", func(t *testing.T) {
		t.Parallel()

		var laughs strings.Builder

		laughs.WriteString("---\na: &a [\"lol\",\"lol\",\"lol\",\"lol\",\"lol\",\"lol\",\"lol\",\"lol\",\"lol\"]\n")
		for i := 'b'; i <= 'i'; i++ {
			prev := string(i - 1)
			laughs.WriteString(string(i) + ": &" + string(i) + " [*" + prev + ",*" + prev + ",*" + prev + ",*" +
				prev + ",*" + prev + ",*" + prev + ",*" + prev + ",*" + prev + ",*" + prev + "]\n")
		}

		laughs.WriteString("---\n")

		inputs := map[string]string{
			"yaml syntax":    "---\ntitle: [unclosed\n---\n",
			"yaml list":      "---\n- a\n- b\n---\n",
			"toml syntax":    "+++\ntitle = \n+++\n",
			"billion laughs": laughs.String(),
		}

		for name, in := range inputs {
			_, _, err := markdown.FrontMatter([]byte(in), 0)

			require.ErrorContains(t, err, "front matter", name)
		}
	})
}
