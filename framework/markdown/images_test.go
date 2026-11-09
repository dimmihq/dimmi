package markdown_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/markdown"
)

func TestImageLayout(t *testing.T) {
	t.Parallel()

	html := func(t *testing.T, layout bool, src string) string {
		t.Helper()
		out, err := markdown.New(markdown.Options{ImageLayout: layout}).HTML([]byte(src))
		require.NoError(t, err)
		return out.HTML
	}

	t.Run("turns the width and alignment into classes and strips them from the address", func(t *testing.T) {
		t.Parallel()

		got := html(t, true, "![Plans](/plans.png#width=60&align=center)")

		assert.Contains(t, got, `src="/plans.png"`)
		assert.Contains(t, got, `class="image-align-center image-width-60"`)
	})

	t.Run("rounds and bounds the width", func(t *testing.T) {
		t.Parallel()

		assert.Contains(t, html(t, true, "![a](/a.png#width=62)"), `class="image-width-60"`)
		assert.Contains(t, html(t, true, "![a](/a.png#width=3)"), `class="image-width-20"`)
		assert.Contains(t, html(t, true, "![a](/a.png#width=400)"), `class="image-width-100"`)
	})

	t.Run("leaves any other fragment in the address", func(t *testing.T) {
		t.Parallel()

		for _, src := range []string{"![a](/a.png#top)", "![a](/a.png#width=wide)", "![a](/a.png#align=left)"} {
			got := html(t, true, src)
			assert.NotContains(t, got, "class=", src)
			assert.Contains(t, got, "#", src)
		}
	})

	t.Run("keeps the fragment when layout is off", func(t *testing.T) {
		t.Parallel()

		got := html(t, false, "![a](/a.png#width=60)")

		assert.Contains(t, got, "/a.png#width=60")
		assert.NotContains(t, got, "class=")
	})
}
