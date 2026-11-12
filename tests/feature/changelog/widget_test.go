package changelog_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"
)

func TestChangelogInTheMessenger(t *testing.T) {
	t.Parallel()

	t.Run("shows published entries, newest first, as last published, once the tab is on", func(t *testing.T) {
		t.Parallel()
		l := newLog(t)
		widget := l.d.Owner.GetJSON("/api/v1/widget-settings").AssertOK().String("data.0.id")
		path := "/widget/v1/apps/" + widget + "/changelog"
		old := l.published(t, testkit.JSON{"title": "Older news", "summary": "The first.", "body_markdown": "First."})
		l.d.TC.Clock.Travel(time.Hour)
		l.published(t, testkit.JSON{"title": "Newer news", "body_markdown": "## Faster\n\nSecond."})
		l.entry(t, testkit.JSON{"title": "Secret plans"})
		l.d.Owner.PatchJSON("/api/v1/changelog-entries/"+old, testkit.JSON{"body_markdown": "Unsaid change."}).
			AssertOK()

		off := l.d.TC.Client().GetJSON(path).AssertOK()
		off.AssertJSONPath("data.entries.#", float64(0))

		l.d.Owner.PatchJSON("/api/v1/widget-settings/"+widget, testkit.JSON{"changelog_tab_enabled": true}).AssertOK()
		res := l.d.TC.Client().GetJSON(path).AssertOK()
		res.AssertJSONPath("data.entries.#", float64(2)).
			AssertJSONPath("data.entries.0.title", "Newer news").
			AssertJSONPath("data.entries.0.body.0.kind", "h2").
			AssertJSONPath("data.entries.1.title", "Older news").
			AssertJSONPath("data.entries.1.summary", "The first.").
			AssertJSONPath("data.entries.1.body.0.text", "First.")
		assert.NotEmpty(t, res.String("data.entries.0.published_at"))
		assert.True(t, strings.HasSuffix(res.String("data.url"), "/changelog"), res.String("data.url"))
		assert.NotContains(t, string(res.Body), "Secret plans")
		assert.NotContains(t, string(res.Body), "Unsaid change")
	})
}
