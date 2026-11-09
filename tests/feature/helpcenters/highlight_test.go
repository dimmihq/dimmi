package helpcenters_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"
)

func TestCodeHighlighting(t *testing.T) {
	t.Parallel()

	const body = "Call it like this:\n\n```go\nfunc main() {\n\treturn 42\n}\n```\n"

	t.Run("colors code in published articles, also ones published before highlighting", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		id := c.article(t, testkit.JSON{"title": "Call the API", "body_markdown": body}).String("data.id")
		c.d.Owner.PostJSON("/api/v1/articles/"+id+"/publications", nil).AssertOK()
		assert.Contains(t, string(c.page("/articles/call-the-api").Body), `class="chroma-kd"`)

		old := c.article(t, testkit.JSON{"title": "Old example", "body_markdown": body}).String("data.id")
		c.d.Owner.PostJSON("/api/v1/articles/"+old+"/publications", nil).AssertOK()
		_, err := c.d.TC.Operator().Exec(t.Context(),
			`UPDATE article_versions SET renderer_version = 1, body_html = '<pre><code>plain</code></pre>'
			 WHERE workspace_id = $1 AND title = 'Old example'`, c.d.TC.Workspace.Workspace.ID)
		require.NoError(t, err)
		refresh := testkit.JSON{"settings": testkit.JSON{"home_title": "Refresh"}}
		c.d.Owner.PatchJSON("/api/v1/help-centers/"+c.id, refresh).AssertOK()

		page := string(c.page("/articles/old-example").Body)
		assert.Contains(t, page, `class="chroma-kd"`)
		assert.NotContains(t, page, "<code>plain</code>")
	})

	t.Run("hands the messenger code as colored tokens, still only text", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		id := c.article(t, testkit.JSON{"title": "Call the API", "body_markdown": body}).String("data.id")
		c.d.Owner.PostJSON("/api/v1/articles/"+id+"/publications", nil).AssertOK()
		app := c.d.Owner.GetJSON("/api/v1/widget-settings").AssertOK().String("data.0.id")

		res := c.d.TC.Client().GetJSON("/widget/v1/apps/" + app + "/help").AssertOK()
		res.AssertJSONPath("data.articles.0.body.1.kind", "code").
			AssertJSONPath("data.articles.0.body.1.tokens.0.class", "kd").
			AssertJSONPath("data.articles.0.body.1.tokens.0.text", "func")
		assert.NotContains(t, string(res.Body), "chroma")
	})
}
