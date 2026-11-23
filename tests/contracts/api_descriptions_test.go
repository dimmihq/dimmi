package contracts_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/mcpserver"
	"github.com/dimmihq/dimmi/tests"
)

/*
TestAPIDescriptions keeps the REST API document explained: every
operation says what it is for, and every parameter and every field of
every schema, nested or not, says what it holds. Agents read the API
through these words, in the reference, the SDKs and the MCP tools, so a
new route, request field or resource field is described before it
ships.
*/
func TestAPIDescriptions(t *testing.T) {
	t.Parallel()
	tc := tests.New(t)

	raw, err := tc.App.Router.OpenAPI("/api/v1")
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))

	missing := mcpserver.Unexplained(doc)

	assert.Empty(t, missing, "describe these in app/mcpserver (the catalog for operations and parameters, "+
		"schema_texts*.go for fields):\n"+strings.Join(missing, "\n"))
}
