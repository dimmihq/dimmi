package helpcenters_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/tests"
)

/*
speaks turns on more languages for the help center beside English.
*/
func (c center) speaks(t *testing.T, locales ...string) {
	t.Helper()
	c.d.Owner.PatchJSON("/api/v1/help-centers/"+c.id, testkit.JSON{"locales": append([]string{"en"}, locales...)}).
		AssertOK()
}

/*
translate saves and publishes an article's translation.
*/
func (c center) translate(t *testing.T, article, locale string, body testkit.JSON) *testkit.Response {
	t.Helper()
	c.d.Owner.PutJSON("/api/v1/articles/"+article+"/translations/"+locale, body).AssertOK()
	return c.d.Owner.PostJSON("/api/v1/articles/"+article+"/translations/"+locale+"/publications", nil).AssertOK()
}

/*
reader opens public pages with the headers a browser sends.
*/
func (c center) browse(headers ...string) func(path string) *testkit.Response {
	return func(path string) *testkit.Response {
		client := c.d.TC.Client().WithHeader("Accept", "text/html")
		for i := 0; i+1 < len(headers); i += 2 {
			client = client.WithHeader(headers[i], headers[i+1])
		}
		return client.Send("GET", c.address(path), nil)
	}
}

func TestHelpCenterLanguages(t *testing.T) {
	t.Parallel()

	t.Run("offers languages beside the default and keeps the single locale field working", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		one := "/api/v1/help-centers/" + c.id
		c.d.Owner.GetJSON(one).AssertOK().AssertJSONPath("data.locale", "en").
			AssertJSONPath("data.default_locale", "en").AssertJSONPath("data.locales", []any{"en"})

		c.d.Owner.PatchJSON(one, testkit.JSON{"locales": []string{"fr", "pt-br", "fr"}}).AssertOK().
			AssertJSONPath("data.locales", []any{"en", "fr", "pt-BR"})
		c.d.Owner.PatchJSON(one, testkit.JSON{"locales": []string{"en", "xx-nope"}}).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "locale_invalid")

		id := c.published(t, testkit.JSON{"title": "Billing", "body_markdown": "Pay monthly."})
		c.translate(t, id, "fr", testkit.JSON{"title": "Facturation", "body_markdown": "Payez chaque mois."})
		c.d.Owner.PatchJSON(one, testkit.JSON{"default_locale": "fr"}).
			AssertStatus(fhttp.StatusConflict).AssertJSONPath("code", "default_locale_translated")

		c.d.Owner.PatchJSON(one, testkit.JSON{"locale": "de"}).AssertOK().
			AssertJSONPath("data.default_locale", "de").AssertJSONPath("data.locales", []any{"de", "fr", "pt-BR"})
	})

	t.Run("edits, publishes and flags a translation outdated when the source changes", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		c.speaks(t, "fr", "de")
		id := c.published(t, testkit.JSON{"title": "Reset your password", "body_markdown": "Open settings."})
		tr := "/api/v1/articles/" + id + "/translations/fr"

		c.d.Owner.PutJSON("/api/v1/articles/"+id+"/translations/en", testkit.JSON{"title": "Nope"}).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "locale_is_default")
		c.d.Owner.PutJSON("/api/v1/articles/"+id+"/translations/es", testkit.JSON{"title": "No"}).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "locale_not_enabled")
		c.d.Owner.GetJSON(tr).AssertNotFound().AssertJSONPath("code", "translation_not_found")

		c.d.Owner.PutJSON(tr, testkit.JSON{
			"title": "Réinitialiser votre mot de passe", "body_markdown": "Ouvrez les réglages.",
		}).AssertOK().AssertJSONPath("data.status", "draft").AssertJSONPath("data.slug", "reinitialiser-votre-mot-de-passe").
			AssertJSONPath("data.outdated", false)
		c.d.Owner.PutJSON(tr, testkit.JSON{"title": "Plus tard", "lock_version": 99}).
			AssertStatus(fhttp.StatusConflict).AssertJSONPath("code", "translation_changed")
		c.d.Owner.PostJSON(tr+"/publications", nil).AssertOK().AssertJSONPath("data.status", "published")
		c.d.Owner.PostJSON(tr+"/publications", nil).AssertStatus(fhttp.StatusConflict)

		states := "/api/v1/help-centers/" + c.id + "/translation-states"
		c.d.Owner.GetJSON(states+"?state=missing").AssertOK().AssertJSONPath("data.0.locale", "de").
			AssertJSONPath("data.#", float64(1))
		c.d.Owner.GetJSON(states+"?state=outdated").AssertOK().AssertJSONPath("data.#", float64(0))

		c.d.Owner.PatchJSON("/api/v1/articles/"+id, testkit.JSON{"body_markdown": "Open settings, then Security."}).
			AssertOK()
		c.d.Owner.GetJSON(tr).AssertOK().AssertJSONPath("data.outdated", false)
		c.d.Owner.PostJSON("/api/v1/articles/"+id+"/publications", nil).AssertOK()
		c.d.Owner.GetJSON(tr).AssertOK().AssertJSONPath("data.outdated", true)
		c.d.Owner.GetJSON(states+"?state=outdated").AssertOK().AssertJSONPath("data.0.locale", "fr")

		c.d.Owner.PostJSON(tr+"/reviews", nil).AssertOK().AssertJSONPath("data.outdated", false)
		c.d.Owner.GetJSON("/api/v1/articles/"+id+"/translations").AssertOK().AssertJSONPath("data.0.locale", "fr")
		versions := c.d.Owner.GetJSON(tr+"/versions").AssertOK().AssertJSONPath("data.#", float64(1))
		c.d.Owner.GetJSON("/api/v1/articles/"+id+"/versions").AssertOK().AssertJSONPath("data.#", float64(2))
		c.d.Owner.PostJSON("/api/v1/articles/"+id+"/restorations", testkit.JSON{
			"version_id": versions.String("data.0.id"),
		}).AssertNotFound()

		c.d.Owner.Delete(tr+"/publications").AssertOK().AssertJSONPath("data.status", "draft")
		c.d.Owner.Delete(tr).AssertNoContent()
		c.d.Owner.GetJSON(tr).AssertNotFound()
	})

	t.Run("keeps translations to their workspace", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		c.speaks(t, "fr")
		id := c.published(t, testkit.JSON{"title": "Private", "body_markdown": "Ours."})
		c.translate(t, id, "fr", testkit.JSON{"title": "Privé", "body_markdown": "À nous."})

		other := tests.NewDesk(t)
		other.Owner.GetJSON("/api/v1/articles/" + id + "/translations/fr").AssertNotFound()
		other.Owner.PutJSON("/api/v1/articles/"+id+"/translations/fr", testkit.JSON{"title": "Hijack"}).
			AssertNotFound()
		other.Owner.GetJSON("/api/v1/help-centers/" + c.id + "/translation-states").AssertNotFound()
	})

	t.Run("serves each language under its prefix and keeps every old address", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		c.speaks(t, "fr", "de")
		col := c.d.Owner.PostJSON("/api/v1/help-centers/"+c.id+"/collections", testkit.JSON{"name": "Account"}).
			AssertCreated().String("data.id")
		empty := c.d.Owner.PostJSON("/api/v1/help-centers/"+c.id+"/collections", testkit.JSON{"name": "Billing"}).
			AssertCreated().String("data.id")
		reset := c.published(t, testkit.JSON{
			"title": "Reset your password", "body_markdown": "Open settings.", "collection_id": col,
		})
		c.published(t, testkit.JSON{"title": "Close your account", "body_markdown": "Write to us.", "collection_id": col})
		c.translate(t, reset, "fr", testkit.JSON{"title": "Réinitialiser le mot de passe", "body_markdown": "Ouvrez les réglages."})
		c.d.Owner.PutJSON("/api/v1/collections/"+col+"/translations/fr", testkit.JSON{"name": "Compte"}).AssertOK().
			AssertJSONPath("data.slug", "compte")
		c.d.Owner.PutJSON("/api/v1/collections/"+empty+"/translations/fr", testkit.JSON{"name": "Facturation"}).AssertOK()
		open := c.browse()

		assert.Contains(t, string(open("/articles/reset-your-password").AssertOK().Body), "Open settings.")
		home := string(open("/fr").AssertOK().Body)
		assert.Contains(t, home, `lang="fr"`)
		assert.Contains(t, home, "Compte")
		assert.NotContains(t, home, "Facturation", "a collection with no French article stays hidden")
		assert.NotContains(t, home, "Close your account", "an untranslated article stays hidden")

		page := string(open("/fr/articles/reinitialiser-le-mot-de-passe").AssertOK().Body)
		assert.Contains(t, page, "Ouvrez les réglages.")
		assert.Contains(t, page, `hreflang="en" href="`+c.address("/articles/reset-your-password")+`"`)
		assert.Contains(t, page, `hreflang="x-default"`)
		assert.Contains(t, page, `hreflang="fr" href="`+c.address("/fr/articles/reinitialiser-le-mot-de-passe")+`"`)
		assert.NotContains(t, page, `rel="alternate" hreflang="de"`)
		assert.Contains(t, page, "data-language-switch")
		assert.Contains(t, page, `href="`+c.local("/de")+`?lang=de"`, "an untranslated page leads to the language's home")

		open("/fr/articles/close-your-account").AssertNotFound()
		open("/fr/collections/billing").AssertNotFound()
		assert.Contains(t, string(open("/fr/collections/compte").AssertOK().Body), "Réinitialiser le mot de passe")
		open("/en/articles/reset-your-password").AssertStatus(fhttp.StatusMovedPermanently).
			AssertHeader("Location", c.local("/articles/reset-your-password"))
		c.d.Owner.PatchJSON("/api/v1/help-centers/"+c.id, testkit.JSON{"locales": []string{"en", "fr"}}).AssertOK()
		open("/de").AssertNotFound()

		found := string(open("/fr/search?q=réglages").AssertOK().Body)
		assert.Contains(t, found, "Réinitialiser le mot de passe")
		assert.NotContains(t, string(open("/fr/search?q=settings").AssertOK().Body), "Reset your password")
	})

	t.Run("opens the home page in the browser's language until the reader picks one", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		c.speaks(t, "fr")
		french := c.browse("Accept-Language", "fr-CA,fr;q=0.9,en;q=0.5")
		french("/").AssertStatus(fhttp.StatusFound).AssertHeader("Location", c.local("/fr"))
		french("/articles/anything").AssertNotFound()
		c.browse("Accept-Language", "de-DE,en;q=0.4")("/").AssertOK()

		picked := c.browse("Accept-Language", "fr", "Cookie", "dimmi_hc_lang=en")
		picked("/").AssertOK()
		res := french("/?lang=en").AssertOK()
		assert.True(t, strings.Contains(res.Header.Get("Set-Cookie"), "dimmi_hc_lang=en"))
	})

	t.Run("lists every language in its own sitemap", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		c.speaks(t, "fr")
		id := c.published(t, testkit.JSON{"title": "Export data", "body_markdown": "Use export."})
		c.published(t, testkit.JSON{"title": "Import data", "body_markdown": "Use import."})
		c.translate(t, id, "fr", testkit.JSON{"title": "Exporter les données", "body_markdown": "Exportez."})

		open := c.browse()
		sitemap := string(open("/sitemap.xml").AssertOK().Body)
		assert.Contains(t, sitemap, "/articles/export-data</loc>")
		assert.NotContains(t, sitemap, "/fr/")
		french := string(open("/fr/sitemap.xml").AssertOK().Body)
		assert.Contains(t, french, "/fr/articles/exporter-les-donnees</loc>")
		assert.NotContains(t, french, "import-data")
		robots := string(open("/robots.txt").AssertOK().Body)
		assert.Contains(t, robots, "/fr/sitemap.xml")
		assert.Contains(t, robots, "Disallow: /fr/search")
		assert.Contains(t, string(open("/fr/llms.txt").AssertOK().Body), "Exporter les données")
	})

	t.Run("redirects a translation's old address in its language", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		c.speaks(t, "fr")
		id := c.published(t, testkit.JSON{"title": "Export data", "body_markdown": "Use export."})
		c.translate(t, id, "fr", testkit.JSON{"title": "Exporter", "body_markdown": "Exportez."})
		c.translate(t, id, "fr", testkit.JSON{"slug": "exporter-les-donnees"})

		open := c.browse()
		open("/fr/articles/exporter").AssertStatus(fhttp.StatusMovedPermanently).
			AssertHeader("Location", c.local("/fr/articles/exporter-les-donnees"))
		open("/articles/exporter").AssertNotFound()
		open("/fr/articles/exporter-les-donnees").AssertOK()
	})

	t.Run("answers the agent in the reader's language when the article is translated", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		c.speaks(t, "fr")
		refunds := c.published(t, testkit.JSON{"title": "Refund policy", "body_markdown": "Refunds take fourteen days."})
		c.published(t, testkit.JSON{"title": "Refund exceptions", "body_markdown": "Refunds for annual plans differ."})
		c.translate(t, refunds, "fr", testkit.JSON{
			"title": "Politique de remboursement", "body_markdown": "Les remboursements prennent quatorze jours.",
		})
		ws := c.d.TC.Workspace.Workspace.ID
		search := func(query, locale string) map[string]string {
			res, err := c.d.TC.App.Knowledge.Search(t.Context(), ws, query, contracts.KnowledgeSearchOptions{
				Locale: locale, Mode: "keyword",
			})
			require.NoError(t, err)
			out := map[string]string{}
			for _, ch := range res.Chunks {
				out[ch.Title] = ch.URL
			}
			return out
		}

		french := search("remboursements refunds", "fr-FR")
		assert.Contains(t, french, "Politique de remboursement")
		assert.NotContains(t, french, "Refund policy", "a translated article answers only in French")
		assert.Contains(t, french["Politique de remboursement"], c.link("/fr/articles/politique-de-remboursement"))
		assert.Contains(t, french, "Refund exceptions", "an untranslated article still answers in its source")

		english := search("refunds", "")
		assert.Contains(t, english, "Refund policy")
		assert.NotContains(t, english, "Politique de remboursement")
	})

	t.Run("shows the messenger's Help tab in the visitor's language", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		c.speaks(t, "fr")
		app := c.d.Owner.GetJSON("/api/v1/widget-settings").AssertOK().String("data.0.id")
		id := c.published(t, testkit.JSON{"title": "Export data", "body_markdown": "Use export."})
		c.published(t, testkit.JSON{"title": "Import data", "body_markdown": "Use import."})
		c.translate(t, id, "fr", testkit.JSON{"title": "Exporter les données", "body_markdown": "Exportez."})

		help := "/widget/v1/apps/" + app + "/help"
		french := c.d.TC.Client().WithHeader("Accept-Language", "fr-FR,fr;q=0.9").GetJSON(help).AssertOK()
		french.AssertJSONPath("data.locale", "fr").AssertJSONPath("data.articles.#", float64(1)).
			AssertJSONPath("data.articles.0.title", "Exporter les données")
		assert.True(t, strings.HasSuffix(french.String("data.url"), "/fr"))
		c.d.TC.Client().GetJSON(help+"?locale=en").AssertOK().AssertJSONPath("data.articles.#", float64(2)).
			AssertJSONPath("data.locale", "en")
	})
}
