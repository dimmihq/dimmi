package knowledge_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/integrations"
	"github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
githubAPI is the GitHub API the repository source reads.
*/
const githubAPI = "https://api.github.com"

/*
serveGitHub answers a GitHub account reaching one private repository
with a main and a develop branch, holding a guide, a nested how-to and a
README beside a Go file.
*/
func serveGitHub(t *testing.T, x index) {
	t.Helper()

	fake := httpclienttest.New(x.tc.App.HTTP).PreventStrayRequests()
	fake.Stub("GET "+githubAPI+"/user", jsonReply(map[string]any{"id": 42, "login": "acme"}))
	fake.Stub("GET "+githubAPI+"/user/repos?per_page=100&sort=updated&page=1", httpclient.Response{
		Status: 200, Header: textHeader("application/json"),
		Body: []byte(`[{"full_name":"acme/docs","private":true}]`),
	})
	fake.Stub("GET "+githubAPI+"/repos/acme/docs", jsonReply(map[string]any{
		"full_name": "acme/docs", "default_branch": "main",
	}))
	fake.Stub("GET "+githubAPI+"/repos/acme/docs/branches?per_page=100&page=1", httpclient.Response{
		Status: 200, Header: textHeader("application/json"), Body: []byte(`[{"name":"main"},{"name":"develop"}]`),
	})
	fake.Stub("GET "+githubAPI+"/repos/acme/docs/git/trees/main?recursive=1", jsonReply(map[string]any{
		"tree": []any{
			map[string]any{"path": "README.md", "type": "blob", "sha": "r1"},
			map[string]any{"path": "main.go", "type": "blob", "sha": "g1"},
			map[string]any{"path": "guides", "type": "tree", "sha": "t1"},
			map[string]any{"path": "guides/refunds.md", "type": "blob", "sha": "f1"},
			map[string]any{"path": "guides/how-to/billing.md", "type": "blob", "sha": "b1"},
		},
	}))
	for path, body := range map[string]string{
		"guides/refunds.md":        "# Refund policy\n\nRefunds are paid within fourteen days.",
		"guides/how-to/billing.md": "# Change your plan\n\nUpgrade from the billing page.",
		"README.md":                "# Acme docs\n\nThe docs of Acme.",
	} {
		fake.Stub("GET "+githubAPI+"/repos/acme/docs/contents/"+path+"?ref=main", httpclient.Response{
			Status: 200, Header: textHeader("text/markdown"), Body: []byte(body),
		})
	}
}

/*
connectGitHub connects the workspace's GitHub account and returns the
installation's public id.
*/
func connectGitHub(t *testing.T, x index) string {
	t.Helper()

	ws := x.tc.Workspace
	owner := authz.NewMembership(ws.Workspace, ws.Owner, auth.RolePermissions(enums.RoleOwner))
	row, err := x.tc.App.Actions.Integrations.CreateManual.Handle(t.Context(), integrations.ManualInstallationInput{
		Membership: owner, Provider: "github", Token: "gho_" + strings.Repeat("k", 36),
	})
	require.NoError(t, err)

	return models.PublicID(models.PrefixIntegrationInstallation, row.ID)
}

func TestGitHubSource(t *testing.T) {
	t.Parallel()

	t.Run("the repository and branch are picked from the connected account", func(t *testing.T) {
		t.Parallel()

		x := newSources(t)
		serveGitHub(t, x)
		installation := connectGitHub(t, x)

		repos := x.owner.PostJSON("/api/v1/knowledge/sources/options", testkit.JSON{
			"type": "github", "field": "repository", "settings": testkit.JSON{"installation": installation},
		}).AssertOK()
		assert.Equal(t, []string{"acme/docs"}, stringsOf(t, repos, "data.options.#.value"))
		assert.Equal(t, "Private", repos.String("data.options.0.detail"))

		branches := x.owner.PostJSON("/api/v1/knowledge/sources/options", testkit.JSON{
			"type": "github", "field": "branch",
			"settings": testkit.JSON{"installation": installation, "repository": "acme/docs"},
		}).AssertOK()
		assert.Equal(t, []string{"main", "develop"}, stringsOf(t, branches, "data.options.#.value"))
		branches.AssertJSONPath("data.options.0.default", true)
	})

	t.Run("a setting without values to pick is refused", func(t *testing.T) {
		t.Parallel()

		x := newSources(t)

		x.owner.PostJSON("/api/v1/knowledge/sources/options", testkit.JSON{
			"type": "website", "field": "start_url", "settings": testkit.JSON{},
		}).AssertStatus(422)
	})

	t.Run("only the ticked folders of the branch are read", func(t *testing.T) {
		t.Parallel()

		x := newSources(t)
		serveGitHub(t, x)
		installation := connectGitHub(t, x)
		settings := testkit.JSON{"installation": installation, "repository": "acme/docs", "branch": "main"}

		tree := x.owner.PostJSON("/api/v1/knowledge/sources/trees", testkit.JSON{
			"type": "github", "settings": settings,
		}).AssertOK()
		assert.Equal(t, []string{"guides", "README.md"}, stringsOf(t, tree, "data.nodes.#.title"))
		assert.Equal(t, "dir:guides", tree.String("data.nodes.0.id"))
		assert.Equal(t, "how-to", tree.String("data.nodes.0.children.0.title"))

		settings["selected"] = []string{"dir:guides"}
		source := x.owner.PostJSON("/api/v1/knowledge/sources", testkit.JSON{
			"type": "github", "name": "Docs", "mode": "sync", "targets": []string{"agent"},
			"settings": settings, "sync_now": true,
		}).AssertCreated().String("data.id")
		settle(t, x, source).AssertJSONPath("data.last_run.status", "succeeded")

		statuses := documentStatuses(t, x, source)
		assert.Equal(t, "active", statuses["Refund policy"])
		assert.Equal(t, "active", statuses["Change your plan"])
		assert.NotContains(t, statuses, "Acme docs", "a file outside the ticked folder is not read")
	})
}
