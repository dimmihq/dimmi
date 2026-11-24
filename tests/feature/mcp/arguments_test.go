package mcp_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

/*
article creates a help article called Refunds and returns its id.
*/
func article(t *testing.T, d tests.Desk, body string) string {
	t.Helper()
	center := d.Owner.GetJSON("/api/v1/help-centers").AssertOK().String("data.0.id")

	return d.Owner.PostJSON("/api/v1/help-centers/"+center+"/articles", testkit.JSON{
		"title": "Refunds", "body_markdown": body,
	}).AssertCreated().String("data.id")
}

func TestArguments(t *testing.T) {
	t.Parallel()

	t.Run("names every argument as the API does and explains it", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		_, token := d.TC.MintToken(d.TC.Workspace.Owner, "*")

		tools, _, err := agent(t, d, token).ListTools(t.Context(), "")
		require.NoError(t, err)

		schemas := map[string]string{}
		for _, tool := range tools {
			schemas[tool.Name] = string(tool.InputSchema)
		}

		assert.Contains(t, schemas["dimmi_get_article_by_id"], `"required":["article"]`)
		assert.Contains(t, schemas["dimmi_get_article_by_id"], "starting with art_")
		assert.Contains(t, schemas["dimmi_create_tag"], `"name"`)
		assert.Contains(t, schemas["dimmi_apply_operator_suggestion"], `"changeset"`)
		assert.Contains(t, schemas["dimmi_list_conversations"], `"filter[team]"`)
		assert.Contains(t, schemas["dimmi_list_conversations"], `"q"`)
		assert.Contains(t, schemas["dimmi_create_ticket"], "Id of the teammate (workspace member)")
		assert.Contains(t, schemas["dimmi_update_tag"], "The name of the tag")
	})

	t.Run("reads an article by its id", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		id := article(t, d, "Refunds take 5 days.")
		_, token := d.TC.MintToken(d.TC.Workspace.Owner, "*")

		res, err := agent(t, d, token).CallTool(t.Context(), "dimmi_get_article_by_id",
			map[string]any{"article": id})
		require.NoError(t, err)

		assert.False(t, res.IsError, res.Text)
		assert.Contains(t, res.Text, "Refunds take 5 days.")
	})

	t.Run("still reads an argument sent as article_id or id", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		id := article(t, d, "Refunds take 5 days.")
		_, token := d.TC.MintToken(d.TC.Workspace.Owner, "*")
		c := agent(t, d, token)

		for _, name := range []string{"article_id", "id"} {
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
		assert.Contains(t, res.Text, `"article" is required and missing`)
		assert.Contains(t, res.Text, "starting with art_")
		assert.Contains(t, res.Text, "dimmi_list_help_center_articles")
	})

	t.Run("names an argument the tool does not have and the one meant", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		id := article(t, d, "Refunds take 5 days.")
		_, token := d.TC.MintToken(d.TC.Workspace.Owner, "*")

		res, err := agent(t, d, token).CallTool(t.Context(), "dimmi_update_article",
			map[string]any{"article": id, "markdown": "New text."})
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
			map[string]any{"article": "Refunds"})
		require.NoError(t, err)

		assert.True(t, res.IsError)
		assert.Contains(t, res.Text, `article: you sent "Refunds". That is not an id`)
		assert.Contains(t, res.Text, "starts with art_")
	})

	t.Run("says in plain words which values an argument takes", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		id := conversation(t, d, "Hello")
		_, token := d.TC.MintToken(d.TC.Workspace.Owner, "*")

		res, err := agent(t, d, token).CallTool(t.Context(), "dimmi_update_conversation",
			map[string]any{"conversation": id, "priority": "whenever"})
		require.NoError(t, err)

		assert.True(t, res.IsError)
		assert.Contains(t, res.Text, `"priority" must be one of none, low, medium, high, urgent; you sent "whenever".`)
	})

	t.Run("says how long a text may be", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		_, token := d.TC.MintToken(d.TC.Workspace.Owner, "*")

		res, err := agent(t, d, token).CallTool(t.Context(), "dimmi_create_tag",
			map[string]any{"name": strings.Repeat("x", 300)})
		require.NoError(t, err)

		assert.True(t, res.IsError)
		assert.Contains(t, res.Text, `"name" takes at most`)
		assert.Contains(t, res.Text, "you sent 300")
	})

	t.Run("filters contacts by a custom attribute", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		d.Owner.PostJSON("/api/v1/attribute-definitions", testkit.JSON{
			"object": "contact", "key": "plan", "label": "Plan", "data_type": "string",
		}).AssertCreated()
		for email, plan := range map[string]string{"ana@example.com": "pro", "bo@example.com": "free"} {
			d.Owner.PostJSON("/api/v1/contacts", testkit.JSON{
				"email": email, "custom_attributes": testkit.JSON{"plan": plan},
			}).AssertCreated()
		}
		_, token := d.TC.MintToken(d.TC.Workspace.Owner, "*")

		res, err := agent(t, d, token).CallTool(t.Context(), "dimmi_list_contacts",
			map[string]any{"filter[custom.plan]": "pro"})
		require.NoError(t, err)

		assert.False(t, res.IsError, res.Text)
		assert.Contains(t, res.Text, "ana@example.com")
		assert.NotContains(t, res.Text, "bo@example.com")
	})

	t.Run("explains the fields inside an argument", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		_, token := d.TC.MintToken(d.TC.Workspace.Owner, "*")

		tools, _, err := agent(t, d, token).ListTools(t.Context(), "")
		require.NoError(t, err)

		for _, tool := range tools {
			if tool.Name != "dimmi_create_conversation" {
				continue
			}

			var schema struct {
				Properties map[string]struct {
					Properties map[string]struct {
						Description string `json:"description"`
					} `json:"properties"`
				} `json:"properties"`
			}
			require.NoError(t, json.Unmarshal(tool.InputSchema, &schema))
			assert.Contains(t, schema.Properties["contact"].Properties["email"].Description, "@")
		}
	})

	t.Run("names a field an argument does not have, and the fields it has", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		_, token := d.TC.MintToken(d.TC.Workspace.Owner, "*")

		res, err := agent(t, d, token).CallTool(t.Context(), "dimmi_create_conversation", map[string]any{
			"inbox_id": d.InboxID(), "body": "Hello", "author": "contact",
			"contact": map[string]any{"mail": "ana@example.com"},
		})
		require.NoError(t, err)

		assert.True(t, res.IsError)
		assert.Contains(t, res.Text, `"contact.mail" is not a field of "contact".`)
		assert.Contains(t, res.Text, "Its fields: email, external_id, id, name.")
		assert.NotContains(t, res.Text, "validating")
	})

	t.Run("names a field missing inside a list's item", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		id := article(t, d, "Refunds take 5 days.")
		_, token := d.TC.MintToken(d.TC.Workspace.Owner, "*")

		res, err := agent(t, d, token).CallTool(t.Context(), "dimmi_propose_article_edit", map[string]any{
			"article_id": id, "suggestion_title": "Fix", "reason": "Stale.",
			"edits": []map[string]any{{"replace": "10 days"}},
		})
		require.NoError(t, err)

		assert.True(t, res.IsError)
		assert.Contains(t, res.Text, `"edits.find" is required and missing.`)
	})

	t.Run("names the API's refused field by the argument the model sent", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		_, token := d.TC.MintToken(d.TC.Workspace.Owner, "conversations:write", "conversations:read")

		res, err := agent(t, d, token).CallTool(t.Context(), "dimmi_create_tag", map[string]any{"name": " "})
		require.NoError(t, err)

		assert.True(t, res.IsError)
		assert.Contains(t, res.Text, "- name:")
	})
}
