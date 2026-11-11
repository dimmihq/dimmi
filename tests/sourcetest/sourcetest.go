/*
Package sourcetest is the contract suite every knowledge source driver
passes. A driver's test hands Run the driver, its settings and the
recorded answers of the source's API; Run replays them through the fake
HTTP transport and checks the driver describes itself fully, connects,
lists every document once and reads each one as clean Markdown.

Fixtures are recorded from the real service once, with Record, and
committed next to the driver's test; afterwards the suite never leaves
the process.
*/
package sourcetest

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"

	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
	fixtures "github.com/dimmihq/dimmi/tests/fixtures/sources"
)

/*
Case is one driver under test. New builds the driver on the client the
fixtures answer through; Config is its settings; Fixture names the file
of recorded exchanges in tests/fixtures/sources. MaxFetch bounds how
many listed documents are fetched. Scrub maps literals to replace in
recorded answers, such as a person's name, beside email addresses,
which are always replaced. Tree says the driver shows its source as a
tree to choose from, which the suite then reads too.
*/
type Case struct {
	New      func(client *httpclient.Client) sources.Driver
	Config   string
	Fixture  string
	MaxFetch int
	Scrub    map[string]string
	Tree     bool
}

/*
Client returns an HTTP client that allows fakes, for driver tests.
*/
func Client(t *testing.T) *httpclient.Client {
	t.Helper()

	client, err := httpclient.New(httpclient.Config{AllowFakes: true})
	require.NoError(t, err)

	return client
}

/*
Replay answers the client's requests from the case's fixture.
*/
func Replay(t *testing.T, client *httpclient.Client, fixture string) *httpclienttest.Fake {
	t.Helper()

	exchanges, err := fixtures.Exchanges(fixture)
	require.NoError(t, err)

	return httpclienttest.New(client).PreventStrayRequests().Replay(exchanges)
}

/*
Run checks the driver against the contract.
*/
func Run(t *testing.T, c Case) {
	t.Helper()

	client := Client(t)
	Replay(t, client, c.Fixture)
	driver := c.New(client)
	cfg := sources.Config{JSON: json.RawMessage(c.Config)}

	t.Run("describes itself fully", func(t *testing.T) {
		checkDescription(t, driver.Describe())
	})

	t.Run("connects and reads every listed document", func(t *testing.T) {
		result, err := driver.Check(t.Context(), cfg)
		require.NoError(t, err)
		assert.NotEmpty(t, result.Account)

		refs := listAll(t, driver, cfg)
		require.NotEmpty(t, refs)
		fetchSome(t, driver, cfg, refs[:min(max(c.MaxFetch, 1), len(refs))])
	})

	if c.Tree {
		t.Run("shows its tree with every item named once", func(t *testing.T) {
			checkTree(t, driver, cfg)
		})
	}

	t.Run("refuses empty settings", func(t *testing.T) {
		_, err := driver.Check(t.Context(), sources.Config{JSON: json.RawMessage(`{}`)})

		var cfgErr *sources.ConfigError
		assert.ErrorAs(t, err, &cfgErr)
	})
}

/*
checkDescription checks what the engine and the dashboard rely on.
*/
func checkDescription(t *testing.T, d sources.Description) {
	t.Helper()

	assert.Regexp(t, `^[a-z][a-z0-9_]{1,39}$`, d.Type)
	assert.NotEmpty(t, d.Name)
	assert.NotEmpty(t, d.Summary)
	assert.NotEmpty(t, d.Logo)
	assert.NotEmpty(t, d.Modes)
	assert.NotEmpty(t, d.Targets)
	assert.NotEmpty(t, d.Schedules)

	var schema struct {
		Type       string                    `json:"type"`
		Properties map[string]map[string]any `json:"properties"`
		Required   []string                  `json:"required"`
	}

	require.NoError(t, json.Unmarshal(d.Schema, &schema), "the schema is JSON")
	assert.Equal(t, "object", schema.Type)
	assert.NotEmpty(t, schema.Properties)

	for _, name := range schema.Required {
		assert.Contains(t, schema.Properties, name, "required field %s is described", name)
	}

	for name, prop := range schema.Properties {
		assert.NotEmpty(t, prop["title"], "field %s has a title", name)
		assert.NotEmpty(t, prop["type"], "field %s has a type", name)
	}
}

/*
listAll pages through the listing, checking every document is named
once.
*/
func listAll(t *testing.T, d sources.Driver, cfg sources.Config) []sources.Ref {
	t.Helper()

	var refs []sources.Ref

	seen := map[string]bool{}
	cursor := ""

	for range 100 {
		page, err := d.List(t.Context(), cfg, cursor)
		require.NoError(t, err)

		for _, r := range page.Refs {
			require.NotEmpty(t, r.ExternalID)
			require.False(t, seen[r.ExternalID], "%s is listed once", r.ExternalID)
			seen[r.ExternalID] = true
			refs = append(refs, r)
		}

		if page.Next == "" {
			return refs
		}

		require.NotEqual(t, cursor, page.Next, "the cursor moves on")
		cursor = page.Next
	}

	t.Fatal("the listing never ends")

	return nil
}

/*
fetchSome reads the documents and checks each is clean Markdown.
*/
func fetchSome(t *testing.T, d sources.Driver, cfg sources.Config, refs []sources.Ref) {
	t.Helper()

	read := 0

	for _, ref := range refs {
		doc, err := d.Fetch(t.Context(), cfg, ref)

		if _, excluded := errors.AsType[*sources.ExcludedError](err); excluded {
			continue
		}

		require.NoError(t, err, "fetch %s", ref.ExternalID)
		assert.NotEmpty(t, strings.TrimSpace(doc.Title), "%s has a title", ref.ExternalID)
		assert.NotEmpty(t, strings.TrimSpace(doc.Markdown), "%s has a body", ref.ExternalID)
		assert.NotContains(t, doc.Markdown, "<script", "%s carries no scripts", ref.ExternalID)
		assert.Contains(t, []sources.Visibility{sources.VisibilityPublic, sources.VisibilityInternal}, doc.Visibility)
		read++
	}

	assert.Positive(t, read, "at least one document reads")
}

/*
checkTree reads the driver's tree and checks it has items, each with an
id used once and a kind.
*/
func checkTree(t *testing.T, d sources.Driver, cfg sources.Config) {
	t.Helper()

	browser, ok := d.(sources.Browser)
	require.True(t, ok, "the driver shows a tree")

	nodes, err := browser.Tree(t.Context(), cfg)
	require.NoError(t, err)
	require.NotEmpty(t, nodes)

	seen := map[string]bool{}

	var walk func(nodes []sources.Node)
	walk = func(nodes []sources.Node) {
		for _, n := range nodes {
			assert.NotEmpty(t, n.ID)
			assert.NotEmpty(t, n.Kind)
			assert.False(t, seen[n.ID], "item %s is named once", n.ID)
			seen[n.ID] = true
			walk(n.Children)
		}
	}

	walk(nodes)
}
