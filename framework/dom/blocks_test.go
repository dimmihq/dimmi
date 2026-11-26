package dom_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/dom"
)

func TestToBlocksMarkdownReadsTheSourceClasses(t *testing.T) {
	t.Parallel()
	classes := dom.BlockClasses{Button: "cta", Center: "mid", Callout: "note"}

	got, err := dom.ToBlocksMarkdown(`<div class="mid"><a class="cta" href="https://example.com/go">Go</a></div>`+
		`<p class="mid">Centered</p>`+
		`<div class="note" style="background-color: #fdecea"><p>Careful</p></div>`, classes)

	require.NoError(t, err)
	assert.Contains(t, got, `::button[Go]{href="https://example.com/go" align="center"}`)
	assert.Contains(t, got, ":::center\nCentered\n:::")
	assert.Contains(t, got, "> [!")
}

func TestToBlocksMarkdownIgnoresClassesItWasNotGiven(t *testing.T) {
	t.Parallel()

	got, err := dom.ToBlocksMarkdown(`<a class="cta" href="https://example.com/go">Go</a>`, dom.BlockClasses{})

	require.NoError(t, err)
	assert.Equal(t, "[Go](https://example.com/go)", got)
}
