package widget_test

import (
	"testing"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

func TestWidgetHomeLinks(t *testing.T) {
	t.Parallel()

	t.Run("a new widget lists no links", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		owner(tc).GetJSON(settingsPath(t, tc)).AssertOK().AssertJSONPath("data.home_links", []any{})
		tc.Client().GetJSON(configPath(t, tc)).AssertOK().AssertJSONPath("data.home_links", []any{})
	})

	t.Run("saves the links in order and the messenger lists them", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		res := patch(t, tc, testkit.JSON{"home_links": []testkit.JSON{
			{"title": "  Book a call ", "url": "https://cal.example.com/acme", "icon": "calendar"},
			{"title": "Write to us", "url": "mailto:help@example.com"},
		}})

		want := []any{
			map[string]any{"title": "Book a call", "url": "https://cal.example.com/acme", "icon": "calendar"},
			map[string]any{"title": "Write to us", "url": "mailto:help@example.com", "icon": "link"},
		}
		res.AssertJSONPath("data.home_links", want)
		tc.Client().GetJSON(configPath(t, tc)).AssertOK().AssertJSONPath("data.home_links", want)
	})

	t.Run("keeps the links when a change leaves them out and clears them with an empty list", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		patch(t, tc, testkit.JSON{"home_links": []testkit.JSON{{"title": "Docs", "url": "https://docs.example.com"}}})

		patch(t, tc, testkit.JSON{"name": "Docs site"}).AssertJSONPath("data.home_links.0.title", "Docs")
		patch(t, tc, testkit.JSON{"home_links": []testkit.JSON{}}).AssertJSONPath("data.home_links", []any{})
	})

	t.Run("previews links without saving them", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		owner(tc).PostJSON(settingsPath(t, tc)+"/preview", testkit.JSON{
			"home_links": []testkit.JSON{{"title": "Docs", "url": "https://docs.example.com", "icon": "book"}},
		}).AssertOK().AssertJSONPath("data.home_links.0.icon", "book")

		owner(tc).GetJSON(settingsPath(t, tc)).AssertOK().AssertJSONPath("data.home_links", []any{})
	})

	t.Run("says on the field why a link is refused", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		res := owner(tc).PatchJSON(settingsPath(t, tc), testkit.JSON{
			"home_links": []testkit.JSON{{"title": "Docs", "url": "http://docs.example.com"}},
		}).AssertValidationError("home_links.0.url")

		res.AssertJSONPath("code", "validation_failed")
		res.AssertJSONPath(`errors.home_links\.0\.url.0`, "Use an https:// address or a mailto: address.")
	})

	seven := make([]testkit.JSON, 7)
	for i := range seven {
		seven[i] = testkit.JSON{"title": "Link", "url": "https://example.com"}
	}

	for name, c := range map[string]struct {
		links []testkit.JSON
		field string
	}{
		"script address": {[]testkit.JSON{{"title": "Hi", "url": "javascript:alert(1)"}}, "home_links.0.url"},
		"plain web page": {[]testkit.JSON{{"title": "Hi", "url": "http://example.com"}}, "home_links.0.url"},
		"bare word":      {[]testkit.JSON{{"title": "Hi", "url": "example"}}, "home_links.0.url"},
		"empty title":    {[]testkit.JSON{{"title": " ", "url": "https://example.com"}}, "home_links.0.title"},
		"unknown icon": {
			[]testkit.JSON{{"title": "Hi", "url": "https://example.com", "icon": "rocket"}}, "home_links.0.icon",
		},
		"long title": {
			[]testkit.JSON{{"title": "A title that runs well past forty characters", "url": "https://example.com"}},
			"home_links.0.title",
		},
		"seventh link": {seven, "home_links"},
	} {
		t.Run("refuses a "+name, func(t *testing.T) {
			t.Parallel()
			tc := tests.New(t)

			owner(tc).PatchJSON(settingsPath(t, tc), testkit.JSON{"home_links": c.links}).
				AssertStatus(fhttp.StatusUnprocessableEntity).AssertValidationError(c.field)
		})
	}
}
