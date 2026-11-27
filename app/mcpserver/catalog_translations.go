/*
|--------------------------------------------------------------------------
| Tool Catalog: Translations
|--------------------------------------------------------------------------
|
| The entries for a help center's languages: translating an article or a
| collection into a language the help center offers, publishing the
| translation, catching it up with the source, and seeing what is left
| to translate.
|
*/

package mcpserver

import "maps"

/*
translationDocs returns the entries for article and collection
translations.
*/
func translationDocs() map[string]Doc {
	out := articleTranslationDocs()
	maps.Copy(out, translationPublishingDocs())
	maps.Copy(out, collectionTranslationDocs())
	return out
}

/*
translationLocale describes the language a translation is in.
*/
const translationLocale = "A language the help center offers besides its default, as a code such as fr or pt-BR."

/*
articleTranslationDocs returns the entries for reading and writing
an article's translations.
*/
func articleTranslationDocs() map[string]Doc {
	return map[string]Doc{
		"articles.translations.index": {
			Title:   "List an article's translations",
			Use:     "Lists the article's translations into the help center's other languages, without their bodies.",
			Returns: "each translation's language, title, status and whether it is behind the source.",
		},
		"articles.translations.show": {
			Title:   "Get an article's translation",
			Use:     "Reads the article's translation into one language, with its working copy in Markdown.",
			Returns: "the translation with its body_markdown and lock_version.",
			Params:  map[string]string{"locale": translationLocale},
		},
		"articles.translations.update": {
			Title: "Translate an article",
			Use: "Saves the article's translation into a language, starting a draft when there is none. Readers " +
				"of that language keep reading the published translation, or the original, until it is published.",
			Returns: "the translation as it now is, with its new lock_version.",
			Next:    []string{"dimmi_publish_article_translation: make the translation live"},
			Params: map[string]string{
				"locale":        translationLocale,
				"title":         "The translated title, 1 to 150 characters. Needed to start a translation.",
				"slug":          "The translated address; made from the title when a translation starts without one.",
				"description":   "The translated summary shown under the title, at most 300 characters.",
				"body_markdown": "The full translated article in Markdown, at most 256 KB; it replaces the old text.",
				"lock_version":  "The lock_version the translation had when you read it; zero skips the check.",
			},
		},
		"articles.translations.destroy": {
			Title:  "Delete an article's translation",
			Use:    "Deletes the translation; readers of its language read the original again. Its versions stay.",
			Risk:   new(RiskDanger),
			Params: map[string]string{"locale": translationLocale},
		},
	}
}

/*
translationPublishingDocs returns the entries for publishing a
translation, catching it up and its versions.
*/
func translationPublishingDocs() map[string]Doc {
	return map[string]Doc{
		"articles.translations.publications.store": {
			Title: "Publish an article's translation",
			Use: "Makes the translation's working copy live for readers of its language and marks it caught up " +
				"with the source.",
			Returns: "the published translation.",
			Risk:    new(RiskReach),
			Params:  map[string]string{"locale": translationLocale},
		},
		"articles.translations.publications.destroy": {
			Title:   "Unpublish an article's translation",
			Use:     "Takes the translation back to a draft; readers of its language read the original again.",
			Returns: "the translation as a draft.",
			Risk:    new(RiskReach),
			Params:  map[string]string{"locale": translationLocale},
		},
		"articles.translations.reviews.store": {
			Title:   "Mark a translation up to date",
			Use:     "Says the translation still matches the source after the source changed, without publishing it.",
			Returns: "the translation, no longer outdated.",
			Params:  map[string]string{"locale": translationLocale},
		},
		"articles.translations.versions.index": {
			Title:   "List a translation's versions",
			Use:     "Lists the published versions of the article's translation into one language, newest first.",
			Returns: "each version's number, title and when it was published.",
			Params:  map[string]string{"locale": translationLocale},
		},
		"articles.translations.restorations.store": {
			Title:   "Restore a translation's version",
			Use:     "Puts an earlier version of the translation back into its working copy, unpublished.",
			Returns: "the translation with the restored text.",
			Next:    []string{"dimmi_publish_article_translation: make the restored text live"},
			Params: map[string]string{
				"locale":     translationLocale,
				"version_id": "Id of one of this translation's versions (artv_...).",
			},
		},
	}
}

/*
collectionTranslationDocs returns the entries for what is left to
translate, collections' translations and the languages on offer.
*/
func collectionTranslationDocs() map[string]Doc {
	return map[string]Doc{
		"help-centers.translation-states.index": {
			Title: "See what is left to translate",
			Use: "Lists where every article of the help center stands in each language it offers besides its " +
				"default: missing, draft or published, and whether it is behind the source.",
			Returns: "one row per article and language.",
			Params: map[string]string{
				"locale": "Only this language, as a code such as fr.",
				"state":  "Only articles in this state: missing, outdated, draft or published.",
			},
		},
		"help-centers.collection-translations.index": {
			Title:   "List a help center's collection translations",
			Use:     "Lists the translated names of every collection of the help center, in every language.",
			Returns: "each collection's name, slug and description in each language it is translated into.",
		},
		"collections.translations.index": {
			Title:   "List a collection's translations",
			Use:     "Lists the collection's name, slug and description in each language it is translated into.",
			Returns: "the collection's translations.",
		},
		"collections.translations.update": {
			Title: "Translate a collection",
			Use: "Saves the collection's name, slug and description in a language the help center offers. It " +
				"shows to readers of that language at once.",
			Returns: "the collection's translation.",
			Risk:    new(RiskReach),
			Params: map[string]string{
				"locale":      translationLocale,
				"name":        "The translated name, 1 to 80 characters.",
				"slug":        "The translated address; made from the name when left out.",
				"description": "The translated description, at most 300 characters.",
			},
		},
		"collections.translations.destroy": {
			Title:  "Delete a collection's translation",
			Use:    "Deletes the collection's translation; readers of its language see its default name again.",
			Risk:   new(RiskDanger),
			Params: map[string]string{"locale": translationLocale},
		},
		"help-center-languages.index": {
			Title:   "List the languages a help center can offer",
			Use:     "Lists every language a help center can be read in, with its name in English and in itself.",
			Returns: "each language's code, name and native name.",
		},
	}
}
