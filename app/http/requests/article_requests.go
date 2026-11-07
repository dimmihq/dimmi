/*
|--------------------------------------------------------------------------
| Article Requests
|--------------------------------------------------------------------------
|
| The inputs of articles. A save names the lock version it started from,
| so two editors never overwrite each other.
|
*/

package requests

/*
IndexArticlesRequest is the input of GET
/help-centers/{help_center}/articles.
*/
type IndexArticlesRequest struct {
	HelpCenter string `path:"help_center"`
	Status     string `query:"status"     enum:"draft,published,archived,"`
	Collection string `query:"collection_id" maxLength:"64"`
	Query      string `query:"query"      maxLength:"200"`
}

/*
ArticleBody is an article's fields. An empty collection_id uncollects
it.
*/
type ArticleBody struct {
	CollectionID *string `json:"collection_id,omitempty" maxLength:"64"`
	Title        *string `json:"title,omitempty"         maxLength:"150"`
	Slug         *string `json:"slug,omitempty"          maxLength:"120"`
	Description  *string `json:"description,omitempty"   maxLength:"300"`
	BodyMarkdown *string `json:"body_markdown,omitempty" maxLength:"262144"`
	Visibility   *string `json:"visibility,omitempty"    enum:"public,agent_only"`
}

/*
StoreArticleRequest is the input of POST
/help-centers/{help_center}/articles.
*/
type StoreArticleRequest struct {
	HelpCenter string `path:"help_center"`
	Body       ArticleBody
}

/*
ArticleRequest names an article in the path.
*/
type ArticleRequest struct {
	Article string `path:"article"`
}

/*
UpdateArticleRequest is the input of PATCH /articles/{article}: a save of
the working copy, or a status of draft or archived.
*/
type UpdateArticleRequest struct {
	Article string `path:"article"`
	Body    struct {
		ArticleBody
		Status      *string `json:"status,omitempty"       enum:"draft,archived"`
		LockVersion int32   `json:"lock_version,omitempty"`
	}
}
