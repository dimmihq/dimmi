/*
|--------------------------------------------------------------------------
| Translation Requests
|--------------------------------------------------------------------------
|
| The inputs of article and collection translations. A translation is
| named by its article or collection and its language, such as fr or
| pt-BR. A save names the lock version it started from, so two editors
| never overwrite each other.
|
*/

package requests

/*
ArticleTranslationRequest names an article's translation in the path.
*/
type ArticleTranslationRequest struct {
	Article string `path:"article"`
	Locale  string `path:"locale"  maxLength:"16"`
}

/*
UpdateArticleTranslationRequest is the input of PUT
/articles/{article}/translations/{locale}: a save of the translation's
working copy, which starts a draft when there is none. A new translation
needs a title.
*/
type UpdateArticleTranslationRequest struct {
	Article string `path:"article"`
	Locale  string `path:"locale"  maxLength:"16"`
	Body    struct {
		Title        *string `json:"title,omitempty"         maxLength:"150"`
		Slug         *string `json:"slug,omitempty"          maxLength:"120"`
		Description  *string `json:"description,omitempty"   maxLength:"300"`
		BodyMarkdown *string `json:"body_markdown,omitempty" maxLength:"262144"`
		LockVersion  int32   `json:"lock_version,omitempty"`
	}
}

/*
RestoreArticleTranslationRequest is the input of POST
/articles/{article}/translations/{locale}/restorations.
*/
type RestoreArticleTranslationRequest struct {
	Article string `path:"article"`
	Locale  string `path:"locale"  maxLength:"16"`
	Body    struct {
		VersionID string `json:"version_id" maxLength:"64"`
	}
}

/*
IndexTranslationStatesRequest is the input of GET
/help-centers/{help_center}/translation-states: every article in each
language, narrowed to one language and to missing or outdated ones when
asked.
*/
type IndexTranslationStatesRequest struct {
	HelpCenter string `path:"help_center"`
	Locale     string `query:"locale" maxLength:"16"`
	State      string `query:"state"  enum:"missing,outdated,draft,published,"`
}

/*
CollectionTranslationRequest names a collection's translation in the
path.
*/
type CollectionTranslationRequest struct {
	Collection string `path:"collection"`
	Locale     string `path:"locale"     maxLength:"16"`
}

/*
UpdateCollectionTranslationRequest is the input of PUT
/collections/{collection}/translations/{locale}.
*/
type UpdateCollectionTranslationRequest struct {
	Collection string `path:"collection"`
	Locale     string `path:"locale"     maxLength:"16"`
	Body       struct {
		Name        string  `json:"name"                  minLength:"1" maxLength:"80"`
		Slug        *string `json:"slug,omitempty"        maxLength:"80"`
		Description *string `json:"description,omitempty" maxLength:"300"`
	}
}
