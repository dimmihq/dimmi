package knowledgesources_test

import (
	"testing"

	"github.com/dimmihq/dimmi/framework/httpclient"

	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources/zendesk"
	"github.com/dimmihq/dimmi/tests/sourcetest"
)

func zendeskCase() sourcetest.Case {
	return sourcetest.Case{
		New:      func(c *httpclient.Client) sources.Driver { return zendesk.New(c) },
		Config:   `{"account":"hivytechnologiesltd","access_token":"test-token"}`,
		Fixture:  "zendesk.json",
		MaxFetch: 3,
		Tree:     true,
		Scrub:    map[string]string{"Frantz Kati": "Ada Admin", "frantz": "ada"},
	}
}

func TestZendesk(t *testing.T) {
	t.Parallel()

	sourcetest.Run(t, zendeskCase())
}

func TestRecordZendesk(t *testing.T) {
	sourcetest.Record(t, zendeskCase(), "ZENDESK_SOURCE_CONFIG", "../../fixtures/sources")
}
