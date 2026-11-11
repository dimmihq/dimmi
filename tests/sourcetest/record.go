package sourcetest

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/filesystem/drivers/local"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
)

/*
Record runs the case against the real service and writes every
exchange to the case's fixture under dir, when the environment holds
DIMMI_RECORD_SOURCES=1; otherwise it skips. The settings come from the
environment variable configEnv, so secrets never land in a file, and
the requests that carried them are not recorded, only the answers.
Point it only at accounts filled with dummy content.
*/
func Record(t *testing.T, c Case, configEnv, dir string) {
	t.Helper()

	if !testkit.Switch("DIMMI_RECORD_SOURCES") {
		t.Skip("set DIMMI_RECORD_SOURCES=1 to record fixtures from the real service")
	}

	client, err := httpclient.New(httpclient.Config{AllowFakes: true})
	require.NoError(t, err)

	rec := httpclienttest.Record(client)
	driver := c.New(client)
	cfg := sources.Config{JSON: json.RawMessage(testkit.Env(t, configEnv))}

	_, err = driver.Check(t.Context(), cfg)
	require.NoError(t, err)

	refs := listAll(t, driver, cfg)
	fetchSome(t, driver, cfg, refs[:min(max(c.MaxFetch, 1), len(refs))])

	if c.Tree {
		checkTree(t, driver, cfg)
	}

	exchanges := rec.Exchanges()
	for i := range exchanges {
		exchanges[i].Body = scrub(exchanges[i].Body, c.Scrub)
	}

	out, err := json.MarshalIndent(exchanges, "", "  ")
	require.NoError(t, err)

	disk, err := local.New(local.Options{Root: dir})
	require.NoError(t, err)
	require.NoError(t, disk.Put(t.Context(), c.Fixture, append(out, '\n'), filesystem.PutOptions{}))
}

/*
emails matches an email address.
*/
var emails = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)

/*
scrub takes what must not reach a public repository out of a recorded
body: every email address, and each literal the case names, such as a
person's name.
*/
func scrub(body string, literals map[string]string) string {
	body = emails.ReplaceAllString(body, "person@example.test")
	for from, to := range literals {
		body = strings.ReplaceAll(body, from, to)
	}

	return body
}
