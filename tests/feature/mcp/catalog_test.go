package mcp_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/mcpserver"
	"github.com/dimmihq/dimmi/tests"
)

func TestCatalog(t *testing.T) {
	t.Parallel()

	tc := tests.New(t)
	report, err := mcpserver.Audit(tc.App.Router)
	require.NoError(t, err)

	t.Run("describes every API operation an agent can call", func(t *testing.T) {
		t.Parallel()
		assert.Empty(t, report.Undocumented, "add catalog entries for:\n"+strings.Join(report.Undocumented, "\n"))
	})

	t.Run("keeps no entry for an operation that is gone", func(t *testing.T) {
		t.Parallel()
		assert.Empty(t, report.Stale)
	})

	t.Run("gives every entry a title and a use", func(t *testing.T) {
		t.Parallel()
		assert.Empty(t, report.Incomplete)
	})

	t.Run("names only tools that exist", func(t *testing.T) {
		t.Parallel()
		assert.Empty(t, report.Unknown)
	})

	t.Run("gives every tool its own name", func(t *testing.T) {
		t.Parallel()
		assert.Empty(t, report.Duplicate)
	})

	t.Run("leaves no argument with only what its name and type say", func(t *testing.T) {
		t.Parallel()
		assert.Empty(t, report.Unwritten, "write a text in app/mcpserver/argument_texts*.go for:\n"+
			strings.Join(report.Unwritten, "\n"))
	})

	t.Run("finds nothing wrong", func(t *testing.T) {
		t.Parallel()
		assert.True(t, report.Clean())
	})
}

func TestDescriptionsMatchTheAPI(t *testing.T) {
	t.Parallel()
	d := tests.NewDesk(t)
	_, token := d.TC.MintToken(d.TC.Workspace.Owner, "*")

	raw, err := d.TC.App.Router.OpenAPI("/api/v1")
	require.NoError(t, err)
	api := string(raw)
	tools, _, err := agent(t, d, token).ListTools(t.Context(), "")
	require.NoError(t, err)

	schemas := map[string]string{}
	for _, tool := range tools {
		schemas[tool.Name] = string(tool.InputSchema)
	}

	t.Run("says in the API document what a tool says, naming the endpoint for the tool", func(t *testing.T) {
		t.Parallel()
		assert.Contains(t, schemas["dimmi_get_article_by_id"], "Find it with dimmi_list_help_center_articles.")
		assert.Contains(t, api, "Find it with GET /help-centers/{help_center}/articles.")
		assert.NotContains(t, api, "dimmi_list_help_center_articles")
	})

	t.Run("gives a field several operations share one text, the same in each tool", func(t *testing.T) {
		t.Parallel()
		assert.Contains(t, schemas["dimmi_create_ai_action"], "When creating")
		assert.Contains(t, schemas["dimmi_update_ai_action"], "When creating")
	})

	t.Run("explains what a record's fields mean", func(t *testing.T) {
		t.Parallel()
		assert.Contains(t, api, "Send it as cursor to read the next page; null on the last page.")
	})
}
