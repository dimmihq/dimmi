package diff_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/support/diff"
)

/*
side rebuilds the old or the new text from word segments.
*/
func side(segments []diff.Segment, skip diff.Op) string {
	var out strings.Builder
	for _, s := range segments {
		if s.Op != skip {
			out.WriteString(s.Text)
		}
	}

	return out.String()
}

func TestLines(t *testing.T) {
	t.Parallel()

	t.Run("returns no hunks for equal texts", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, diff.Lines("a\nb\n", "a\nb\n"))
		assert.Empty(t, diff.Lines("", ""))
	})

	t.Run("groups a change with three lines of context", func(t *testing.T) {
		t.Parallel()

		a := "1\n2\n3\n4\n5\n6\n7\n8\n9\n"
		b := "1\n2\n3\n4\nfive\n6\n7\n8\n9\n"

		hunks := diff.Lines(a, b)

		require.Len(t, hunks, 1)
		assert.Equal(t, diff.Hunk{
			FromLine: 2,
			ToLine:   2,
			Lines: []diff.Line{
				{Op: diff.Equal, Text: "2\n"},
				{Op: diff.Equal, Text: "3\n"},
				{Op: diff.Equal, Text: "4\n"},
				{Op: diff.Delete, Text: "5\n"},
				{Op: diff.Insert, Text: "five\n"},
				{Op: diff.Equal, Text: "6\n"},
				{Op: diff.Equal, Text: "7\n"},
				{Op: diff.Equal, Text: "8\n"},
			},
		}, hunks[0])
	})

	t.Run("splits distant changes into separate hunks", func(t *testing.T) {
		t.Parallel()

		var a, b strings.Builder
		for i := range 30 {
			line := strings.Repeat("x", i) + "\n"
			a.WriteString(line)

			if i == 2 || i == 25 {
				line = "changed\n"
			}

			b.WriteString(line)
		}

		hunks := diff.Lines(a.String(), b.String())

		require.Len(t, hunks, 2)
		assert.Equal(t, 1, hunks[0].FromLine)
		assert.Equal(t, 23, hunks[1].FromLine)
	})

	t.Run("handles texts without a final newline", func(t *testing.T) {
		t.Parallel()

		hunks := diff.Lines("a", "b")

		require.Len(t, hunks, 1)
		assert.Equal(t, []diff.Line{{Op: diff.Delete, Text: "a"}, {Op: diff.Insert, Text: "b"}}, hunks[0].Lines)
	})

	t.Run("handles an empty side", func(t *testing.T) {
		t.Parallel()

		hunks := diff.Lines("", "new\n")

		require.Len(t, hunks, 1)
		assert.Equal(t, []diff.Line{{Op: diff.Insert, Text: "new\n"}}, hunks[0].Lines)
	})
}

func TestWords(t *testing.T) {
	t.Parallel()

	t.Run("replaces whole words", func(t *testing.T) {
		t.Parallel()

		got := diff.Words("the cat sat", "the cut sat")

		assert.Equal(t, []diff.Segment{
			{Op: diff.Equal, Text: "the "},
			{Op: diff.Delete, Text: "cat"},
			{Op: diff.Insert, Text: "cut"},
			{Op: diff.Equal, Text: " sat"},
		}, got)
	})

	t.Run("returns one equal segment for equal texts", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, []diff.Segment{{Op: diff.Equal, Text: "same text."}}, diff.Words("same text.", "same text."))
		assert.Empty(t, diff.Words("", ""))
	})

	t.Run("treats punctuation as its own token", func(t *testing.T) {
		t.Parallel()

		got := diff.Words("Hello, world", "Hello! world")

		assert.Contains(t, got, diff.Segment{Op: diff.Delete, Text: ","})
		assert.Contains(t, got, diff.Segment{Op: diff.Insert, Text: "!"})
	})

	t.Run("rebuilds both texts from the segments", func(t *testing.T) {
		t.Parallel()

		cases := []struct{ a, b string }{
			{"The quick brown fox jumps.", "A quick red fox leaped over."},
			{"naïve café — 東京 tower", "naive café — 大阪 tower!"},
			{"", "only new"},
			{"only old", ""},
			{"tabs\tand\nnew lines", "tabs and\n\nnew  lines"},
			{"\xff\xfe invalid bytes", "valid bytes"},
		}

		for _, tc := range cases {
			segments := diff.Words(tc.a, tc.b)

			assert.Equal(t, tc.a, side(segments, diff.Insert), tc.a)
			assert.Equal(t, tc.b, side(segments, diff.Delete), tc.b)
		}
	})

	t.Run("keeps many distinct words apart", func(t *testing.T) {
		t.Parallel()

		var a, b strings.Builder
		for i := range 70000 {
			word := strings.Repeat("w", i%7) + string(rune('a'+i%26)) + string(rune(0x4E00+i))
			a.WriteString(word + " ")
			b.WriteString(word + " ")
		}

		b.WriteString("tail")

		segments := diff.Words(a.String(), b.String())

		assert.Equal(t, []diff.Segment{
			{Op: diff.Equal, Text: a.String()},
			{Op: diff.Insert, Text: "tail"},
		}, segments)
	})
}
