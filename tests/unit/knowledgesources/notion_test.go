package knowledgesources_test

import (
	"testing"

	"github.com/dimmihq/dimmi/framework/httpclient"

	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources/notion"
	"github.com/dimmihq/dimmi/tests/sourcetest"
)

func notionCase() sourcetest.Case {
	return sourcetest.Case{
		New:      func(c *httpclient.Client) sources.Driver { return notion.New(c) },
		Config:   `{"access_token":"ntn_test"}`,
		Fixture:  "notion.json",
		Tree:     true,
		MaxFetch: 4,
	}
}

func TestNotion(t *testing.T) {
	t.Parallel()

	sourcetest.Run(t, notionCase())
}

func TestRecordNotion(t *testing.T) {
	sourcetest.Record(t, notionCase(), "NOTION_SOURCE_CONFIG", "../../fixtures/sources")
}
