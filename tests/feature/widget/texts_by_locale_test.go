package widget_test

import (
	"strings"
	"testing"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

func TestWidgetTextsByLocale(t *testing.T) {
	t.Parallel()

	t.Run("a language's own words beat the shared ones, which beat the built-in words", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		res := patch(t, tc, testkit.JSON{
			"texts_by_locale": testkit.JSON{"FR": testkit.JSON{"greeting_intro": "Salut l'équipe"}},
		}).AssertOK()
		res.AssertJSONPath("data.texts_by_locale.fr.greeting_intro", "Salut l'équipe")

		client := tc.Client()
		client.GetJSON(configPath(t, tc)+"?locale=fr").AssertOK().
			AssertJSONPath("data.texts.greeting_intro", "Salut l'équipe")
		client.GetJSON(configPath(t, tc)+"?locale=de").AssertOK().AssertJSONPath("data.texts.greeting_intro", "Hallo!")
		client.GetJSON(configPath(t, tc)+"?locale=en").AssertOK().
			AssertJSONPath("data.texts.greeting_intro", "Hey there.")

		patch(t, tc, testkit.JSON{"texts": testkit.JSON{"greeting_intro": "Welcome"}}).AssertOK()

		client.GetJSON(configPath(t, tc)+"?locale=fr").AssertOK().
			AssertJSONPath("data.texts.greeting_intro", "Salut l'équipe")
		client.GetJSON(configPath(t, tc)+"?locale=de").AssertOK().AssertJSONPath("data.texts.greeting_intro", "Welcome")
		client.GetJSON(configPath(t, tc)+"?locale=en").AssertOK().AssertJSONPath("data.texts.greeting_intro", "Welcome")
	})

	t.Run("clearing a language's word or the whole language reverts it", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		patch(t, tc, testkit.JSON{"texts_by_locale": testkit.JSON{
			"fr": testkit.JSON{"greeting_intro": "Salut", "delay_notice": "Absents"},
			"de": testkit.JSON{"greeting_intro": "Servus"},
		}}).AssertOK()

		res := patch(t, tc, testkit.JSON{"texts_by_locale": testkit.JSON{
			"fr": testkit.JSON{"greeting_intro": ""}, "de": nil,
		}}).AssertOK()
		res.AssertJSONPath("data.texts_by_locale", map[string]any{
			"fr": map[string]any{"delay_notice": "Absents"},
		})

		patch(t, tc, testkit.JSON{"texts_by_locale": testkit.JSON{"fr": testkit.JSON{"delay_notice": nil}}}).
			AssertOK().AssertJSONPath("data.texts_by_locale", map[string]any{})
	})

	t.Run("refuses an unknown key, a long text or a tag that is not a language", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		refuse := func(texts testkit.JSON, field string) {
			t.Helper()
			owner(tc).PatchJSON(settingsPath(t, tc), testkit.JSON{"texts_by_locale": texts}).
				AssertStatus(fhttp.StatusUnprocessableEntity).AssertValidationError(field)
		}

		refuse(testkit.JSON{"fr": testkit.JSON{"nope": "x"}}, "texts_by_locale.fr.nope")
		long := testkit.JSON{"launcher_label": strings.Repeat("a", 25)}
		refuse(testkit.JSON{"fr": long}, "texts_by_locale.fr.launcher_label")
		refuse(testkit.JSON{"not a tag!": testkit.JSON{"greeting": "x"}}, "texts_by_locale.not a tag!")
		refuse(testkit.JSON{"fr": "Salut"}, "texts_by_locale.fr")
	})

	t.Run("the preview shows a language's words before they are saved", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		owner(tc).PostJSON(settingsPath(t, tc)+"/preview?locale=fr", testkit.JSON{
			"texts_by_locale": testkit.JSON{"fr": testkit.JSON{"greeting_intro": "Coucou"}},
		}).AssertOK().AssertJSONPath("data.texts.greeting_intro", "Coucou")

		owner(tc).GetJSON(settingsPath(t, tc)).AssertOK().AssertJSONPath("data.texts_by_locale", map[string]any{})
	})
}
