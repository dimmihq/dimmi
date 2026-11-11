package knowledgesources_test

import (
	"testing"

	"github.com/dimmihq/dimmi/framework/httpclient"

	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources/intercom"
	"github.com/dimmihq/dimmi/tests/sourcetest"
)

func intercomCase() sourcetest.Case {
	return sourcetest.Case{
		New:      func(c *httpclient.Client) sources.Driver { return intercom.New(c) },
		Config:   `{"access_token":"dG9rOnRlc3Q=","region":"us"}`,
		Fixture:  "intercom.json",
		Tree:     true,
		MaxFetch: 3,
		Scrub:    map[string]string{"Frantz Kati": "Ada Admin", "frantz": "ada"},
	}
}

func TestIntercom(t *testing.T) {
	t.Parallel()

	sourcetest.Run(t, intercomCase())
}

func TestRecordIntercom(t *testing.T) {
	sourcetest.Record(t, intercomCase(), "INTERCOM_SOURCE_CONFIG", "../../fixtures/sources")
}
