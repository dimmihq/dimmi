package mcp_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

/*
article creates a help article and returns its id.
*/
func article(t *testing.T, d tests.Desk, title, body string) string {
	t.Helper()
	center := d.Owner.GetJSON("/api/v1/help-centers").AssertOK().String("data.0.id")

	return d.Owner.PostJSON("/api/v1/help-centers/"+center+"/articles", testkit.JSON{
		"title": title, "body_markdown": body,
	}).AssertCreated().String("data.id")
}

func TestArguments(t *testing.T) {
	t.Parallel()

	t.Run("names every argument for what it holds", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		_, token := d.TC.MintToken(d.TC.Workspace.Owner, "*")

		tools, _, err := agent(t, d, token).ListTools(t.Context(), "")
		require.NoError(t, err)

		schemas := map[string]string{}
		for _, tool := range tools {
			schemas[tool.Name] = string(tool.InputSchema)
		}

		assert.Contains(t, schemas["dimmi_get_article_by_id"], `"required":["article_id"]`)
		assert.Contains(t, schemas["dimmi_get_article_by_id"], "starting with art_")
		assert.Contains(t, schemas["dimmi_create_tag"], `"tag_name"`)
		assert.Contains(t, schemas["dimmi_apply_operator_suggestion"], `"suggestion_id"`)
	})

	t.Run("reads an article by its id", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		id := article(t, d, "Refunds", "Refunds take 5 days.")
		_, token := d.TC.MintToken(d.TC.Workspace.Owner, "*")

		res, err := agent(t, d, token).CallTool(t.Context(), "dimmi_get_article_by_id",
			map[string]any{"article_id": id})
		require.NoError(t, err)

		assert.False(t, res.IsError, res.Text)
		assert.Contains(t, res.Text, "Refunds take 5 days.")
	})

	t.Run("still reads an argument sent under the API's name or as id", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		id := article(t, d, "Refunds", "Refunds take 5 days.")
		_, token := d.TC.MintToken(d.TC.Workspace.Owner, "*")
		c := agent(t, d, token)

		for _, name := range []string{"article", "id"} {
			res, err := c.CallTool(t.Context(), "dimmi_get_article_by_id", map[string]any{name: id})
			require.NoError(t, err)
			assert.False(t, res.IsError, res.Text)
		}
	})

	t.Run("reads a conversation's number sent as a number", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		conversation(t, d, "Hello")
		_, token := d.TC.MintToken(d.TC.Workspace.Owner, "conversations:read")

		res, err := agent(t, d, token).CallTool(t.Context(), "dimmi_get_conversation_by_id_or_number",
			map[string]any{"conversation_number": 1})
		require.NoError(t, err)

		assert.False(t, res.IsError, res.Text)
		assert.Contains(t, res.Text, "conv_")
	})

	t.Run("names a missing argument and what belongs in it", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		_, token := d.TC.MintToken(d.TC.Workspace.Owner, "*")

		res, err := agent(t, d, token).CallTool(t.Context(), "dimmi_get_article_by_id", map[string]any{})
		require.NoError(t, err)

		assert.True(t, res.IsError)
		assert.Contains(t, res.Text, `"article_id" is required and missing`)
		assert.Contains(t, res.Text, "starting with art_")
		assert.Contains(t, res.Text, "dimmi_list_help_center_articles")
	})

	t.Run("names an argument the tool does not have and the one meant", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		id := article(t, d, "Refunds", "Refunds take 5 days.")
		_, token := d.TC.MintToken(d.TC.Workspace.Owner, "*")

		res, err := agent(t, d, token).CallTool(t.Context(), "dimmi_update_article",
			map[string]any{"article_id": id, "markdown": "New text."})
		require.NoError(t, err)

		assert.True(t, res.IsError)
		assert.Contains(t, res.Text, `"markdown" is not an argument of this tool. Did you mean "body_markdown"?`)
		assert.Contains(t, res.Text, "Nothing was changed")
	})

	t.Run("says a title sent where an id belongs is not an id", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		_, token := d.TC.MintToken(d.TC.Workspace.Owner, "*")

		res, err := agent(t, d, token).CallTool(t.Context(), "dimmi_get_article_by_id",
			map[string]any{"article_id": "Refunds"})
		require.NoError(t, err)

		assert.True(t, res.IsError)
		assert.Contains(t, res.Text, `article_id: you sent "Refunds". That is not an id`)
		assert.Contains(t, res.Text, "starts with art_")
	})

	t.Run("names the API's refused field by the argument the model sent", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		_, token := d.TC.MintToken(d.TC.Workspace.Owner, "conversations:write", "conversations:read")

		res, err := agent(t, d, token).CallTool(t.Context(), "dimmi_create_tag", map[string]any{"tag_name": " "})
		require.NoError(t, err)

		assert.True(t, res.IsError)
		assert.Contains(t, res.Text, "- tag_name:")
	})
}
