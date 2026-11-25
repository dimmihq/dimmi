package prompts

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/enums"
)

func TestTemplateGuidance(t *testing.T) {
	t.Parallel()

	t.Run("gives the sales template its written guidance, without the editors' note", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, CheckGuidance())
		text, ok := TemplateGuidance("sales")
		require.True(t, ok)
		assert.True(t, strings.HasPrefix(text, "## "), "it opens on its first section")
		assert.NotContains(t, text, "<!--", "the note for editors is never part of the guidance")
		assert.NotContains(t, text, "{{", "guidance is plain text, with nothing left to fill in")
	})

	t.Run("has none for a template without a file, or a key that is not a template", func(t *testing.T) {
		t.Parallel()
		for _, key := range []string{"", "no-such-template", "../agent/v2/anthropic/job", "sales.md", "a/b"} {
			_, ok := TemplateGuidance(key)
			assert.False(t, ok, key)
		}
	})
}

func TestSalesRoot(t *testing.T) {
	t.Parallel()

	const marker = "THE-TEAMS-GUIDANCE-GOES-HERE"
	l, err := NewLoader("", nil)
	require.NoError(t, err)

	for _, family := range []enums.AIModelFamily{
		enums.AIModelFamilyAnthropic, enums.AIModelFamilyOpenAI, enums.AIModelFamilyOpenWeight,
	} {
		name := "puts the team's guidance after everything of ours but the last check, for " + string(family)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			file, err := l.Render("v2", family, RootSales, Vars{
				"agent_name": "Ada", "product_name": "Lovelace", "tone": "Warm.", "language": "German",
				"guidance": marker, "channel": "chat", "handoff": "available", "rules": "No extra rules.",
				"job": "", "transfers": "",
			})
			require.NoError(t, err)
			text := file.Content
			assert.NotContains(t, text, "{{", "every placeholder is filled")
			require.Equal(t, 1, strings.Count(text, marker), "the guidance is written once")

			at := strings.Index(text, marker)
			assert.Greater(t, at, strings.LastIndex(text, "</example>"), "it comes after every worked example")
			after := text[at+len(marker):]
			assert.NotContains(t, after, "<example>")
			assert.Less(t, len(after), len(text)/8, "only the short check before replying follows it")
		})
	}
}
