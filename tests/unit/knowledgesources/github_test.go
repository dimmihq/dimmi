package knowledgesources_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"

	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources/github"
	"github.com/dimmihq/dimmi/tests/sourcetest"
)

func githubCase() sourcetest.Case {
	return sourcetest.Case{
		New:      func(c *httpclient.Client) sources.Driver { return github.New(c) },
		Config:   `{"repository":"github/gitignore","branch":"main","access_token":"test-token"}`,
		Fixture:  "github.json",
		MaxFetch: 3,
		Tree:     true,
	}
}

func TestGitHub(t *testing.T) {
	t.Parallel()

	sourcetest.Run(t, githubCase())
}

func TestRecordGitHub(t *testing.T) {
	sourcetest.Record(t, githubCase(), "GITHUB_SOURCE_CONFIG", "../../fixtures/sources")
}

func TestGitHubChoices(t *testing.T) {
	t.Parallel()

	reply := func(body string) httpclient.Response {
		return httpclient.Response{
			Status: 200, Body: []byte(body), Header: map[string][]string{"Content-Type": {"application/json"}},
		}
	}

	newDriver := func(t *testing.T) *github.Driver {
		t.Helper()

		client := sourcetest.Client(t)
		fake := httpclienttest.New(client).PreventStrayRequests()
		fake.Stub("GET "+github.API+"/user/repos?per_page=100&sort=updated&page=1",
			reply(`[{"full_name":"acme/docs","private":true},{"full_name":"acme/site","private":false}]`))
		fake.Stub("GET "+github.API+"/repos/acme/docs", reply(`{"full_name":"acme/docs","default_branch":"main"}`))
		fake.Stub("GET "+github.API+"/repos/acme/docs/branches?per_page=100&page=1",
			reply(`[{"name":"develop"},{"name":"main"}]`))

		return github.New(client)
	}

	cfg := func(raw string) sources.Config { return sources.Config{JSON: json.RawMessage(raw)} }

	t.Run("repositories are listed from the connected account", func(t *testing.T) {
		t.Parallel()

		options, err := newDriver(t).Options(t.Context(), cfg(`{"access_token":"t"}`), "repository")
		require.NoError(t, err)
		require.Len(t, options, 2)
		assert.Equal(t, sources.Option{Value: "acme/docs", Label: "acme/docs", Detail: "Private"}, options[0])
		assert.Equal(t, "Public", options[1].Detail)
	})

	t.Run("branches put the default one first and preselect it", func(t *testing.T) {
		t.Parallel()

		settings := cfg(`{"access_token":"t","repository":"acme/docs"}`)
		options, err := newDriver(t).Options(t.Context(), settings, "branch")
		require.NoError(t, err)
		require.Len(t, options, 2)
		assert.True(t, options[0].Default)
		assert.Equal(t, "main", options[0].Value)
		assert.Equal(t, "develop", options[1].Value)
	})

	t.Run("nothing is listed without a connected account or for other settings", func(t *testing.T) {
		t.Parallel()

		var cfgErr *sources.ConfigError

		_, err := newDriver(t).Options(t.Context(), cfg(`{}`), "repository")
		require.ErrorAs(t, err, &cfgErr)
		assert.Equal(t, "installation", cfgErr.Field)

		_, err = newDriver(t).Options(t.Context(), cfg(`{"access_token":"t"}`), "branch")
		require.ErrorAs(t, err, &cfgErr)
		assert.Equal(t, "repository", cfgErr.Field)

		_, err = newDriver(t).Options(t.Context(), cfg(`{"access_token":"t","repository":"acme/docs"}`), "shade")
		require.ErrorAs(t, err, &cfgErr)
	})
}
