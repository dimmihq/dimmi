package knowledgesources_test

import (
	"encoding/json"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/filesystem/drivers/local"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources/website"
	fixtures "github.com/dimmihq/dimmi/tests/fixtures/sources"
	"github.com/dimmihq/dimmi/tests/sourcetest"
)

/*
accountPath finds the account id in a Cloudflare address.
*/
var accountPath = regexp.MustCompile(`/accounts/[0-9a-f]{32}/`)

/*
cloudflareSite is the site the Cloudflare crawl reads.
*/
const cloudflareSite = `{"start_url":"https://go.dev/doc/","max_pages":5}`

func cloudflareDriver(client *httpclient.Client, token string) *website.Driver {
	builtin := website.NewBuiltin(client, clock.System(), 0)

	cf := &website.Cloudflare{HTTP: client, Clock: clock.System(), Token: token, AccountID: "acct"}

	return website.New(builtin).Through(cf)
}

/*
crawl lists the whole site, waiting out the running job, and returns
the pages with the browser time the crawl reported.
*/
func crawl(t *testing.T, d *website.Driver, wait time.Duration) ([]sources.Ref, int64) {
	t.Helper()

	cfg := sources.Config{JSON: json.RawMessage(cloudflareSite)}
	cursor := ""

	var (
		refs      []sources.Ref
		browserMs int64
	)

	for range 200 {
		page, err := d.List(t.Context(), cfg, cursor)
		if _, ok := errors.AsType[*sources.WaitError](err); ok {
			<-clock.System().After(wait)

			continue
		}

		require.NoError(t, err)
		refs = append(refs, page.Refs...)
		browserMs += page.Usage.BrowserMs

		if page.Next == "" {
			return refs, browserMs
		}

		cursor = page.Next
	}

	t.Fatal("the crawl never finished")

	return nil, 0
}

func TestCloudflareCrawler(t *testing.T) {
	t.Parallel()

	t.Run("a crawl job is started, waited for and read with every page in markdown", func(t *testing.T) {
		t.Parallel()

		client := sourcetest.Client(t)
		exchanges, err := fixtures.Exchanges("cloudflare.json")
		require.NoError(t, err)
		httpclienttest.New(client).PreventStrayRequests().Replay(exchanges)

		refs, browserMs := crawl(t, cloudflareDriver(client, "cf-test"), 0)
		require.NotEmpty(t, refs)
		assert.Equal(t, int64(569), browserMs, "the browser time Cloudflare reported is passed on once")

		for _, r := range refs {
			require.NotNil(t, r.Document, "%s comes with its page", r.URL)
			assert.NotEmpty(t, r.Document.Markdown)
			assert.NotEmpty(t, r.Version)
			assert.Contains(t, r.URL, "https://go.dev/")
		}
	})
}

func TestRecordCloudflare(t *testing.T) {
	if !testkit.Switch("DIMMI_RECORD_SOURCES") {
		t.Skip("set DIMMI_RECORD_SOURCES=1 and CLOUDFLARE_API_TOKEN to crawl for real")
	}

	client, err := httpclient.New(httpclient.Config{AllowFakes: true, Timeout: 2 * time.Minute})
	require.NoError(t, err)

	rec := httpclienttest.Record(client)
	d := website.New(website.NewBuiltin(client, clock.System(), 0)).
		Through(&website.Cloudflare{
			HTTP: client, Clock: clock.System(), Token: testkit.Env(t, "CLOUDFLARE_API_TOKEN"),
			MaxWait: 90 * time.Second,
		})

	started := clock.System().Now()
	refs, _ := crawl(t, d, 10*time.Second)
	t.Logf("crawled %d pages in %s", len(refs), clock.System().Now().Sub(started).Round(time.Second))

	for _, r := range refs {
		t.Logf("%s | %s | %d bytes", r.URL, r.Title, len(r.Document.Markdown))
	}

	doc, err := d.Fetch(t.Context(), sources.Config{JSON: json.RawMessage(cloudflareSite)}, refs[0])
	require.NoError(t, err)
	t.Logf("single page %s: %d bytes", doc.Title, len(doc.Markdown))

	out := rec.Exchanges()
	for i := range out {
		out[i].URL = accountPath.ReplaceAllString(out[i].URL, "/accounts/acct/")
	}

	raw, err := json.MarshalIndent(out, "", "  ")
	require.NoError(t, err)

	disk, err := local.New(local.Options{Root: "../../fixtures/sources"})
	require.NoError(t, err)
	require.NoError(t, disk.Put(t.Context(), "cloudflare.json", append(raw, '\n'), filesystem.PutOptions{}))
}
