/*
|--------------------------------------------------------------------------
| Schema Texts: Translations
|--------------------------------------------------------------------------
|
| The fields of a help center's languages and of the translations of its
| articles and collections.
|
*/

package mcpserver

/*
schemaTextsFromTranslation returns the texts of translation schemas and
of the help center's language fields.
*/
func schemaTextsFromTranslation() map[string]map[string]string {
	return map[string]map[string]string{
		"ArticleTranslation": {
			"article_id":    "The id of the article translated, such as art_abc123.",
			"locale":        "The language of the translation, as a code such as fr or pt-BR.",
			"title":         "The translated title.",
			"slug":          "The translation's address in its language, such as remboursements.",
			"description":   "The translated summary shown under the title. Null when it has none.",
			"body_markdown": "The translation's working copy in Markdown. Left out of lists.",
			"status":        "draft until the translation is published, then published.",
			"has_unpublished_changes": "True when the working copy differs from the version readers of the " +
				"language see.",
			"outdated":     "True when the source was published with other words since the translation caught up.",
			"lock_version": "Goes up with every save; send it back to save over this version only.",
			"word_count":   "The number of words in the translation's working copy.",
			"published_at": "When the translation was last published. Null while it never was.",
		},
		"CollectionTranslation": {
			"collection_id": "The id of the collection translated, such as col_abc123.",
			"locale":        "The language of the translation, as a code such as fr.",
			"name":          "The collection's name in that language.",
			"slug":          "The collection's address in that language.",
			"description":   "The collection's description in that language. Null when it has none.",
			"outdated":      "True when the collection's default name or description changed since.",
		},
		"TranslationState": {
			"article_id":    "The id of the article, such as art_abc123.",
			"collection_id": "The id of the article's collection. Null when it is in none.",
			"title":         "The article's title in the help center's default language.",
			"status":        "The article's own status, such as published.",
			"locale":        "The language this row is about, as a code such as fr.",
			"state":         "Where the article stands in that language: missing, draft or published.",
			"outdated":      "True when the translation is behind the source.",
		},
		"Language": {
			"code":        "The language's code, such as fr or pt-BR.",
			"name":        "The language's name in English, such as French.",
			"native_name": "The language's name in itself, such as Français.",
		},
	}
}

/*
schemaTextsFromHelpCenterLanguages returns the texts of the help center's
language fields.
*/
func schemaTextsFromHelpCenterLanguages() map[string]map[string]string {
	return map[string]map[string]string{
		"HelpCenter": {
			"default_locale": "The language articles are written in, as a code such as en. Same as locale.",
			"locales": "Every language the help center is read in, the default first. Readers of another " +
				"language read it under /<code>/.",
		},
		"HelpCenterSettings": {
			"untranslated_articles": "What readers of another language see of an article not translated " +
				"into it: original shows it in the default language, hide leaves it out.",
		},
		"HelpCenterSettingsBody": {
			"untranslated_articles": "What readers of another language see of an article not translated " +
				"into it: original shows it in the default language, hide leaves it out.",
		},
	}
}
