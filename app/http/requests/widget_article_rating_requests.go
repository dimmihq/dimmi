/*
|--------------------------------------------------------------------------
| Widget Article Rating Requests
|--------------------------------------------------------------------------
|
| The messenger says whether an article of its help center helped the
| visitor reading it.
|
*/

package requests

/*
StoreWidgetArticleRatingRequest is the input of POST
/widget/v1/article-ratings: the slug of the article and whether it helped.
*/
type StoreWidgetArticleRatingRequest struct {
	Body struct {
		Article string `json:"article" minLength:"1" maxLength:"200" pattern:"^[a-z0-9-]+$"`
		Helpful bool   `json:"helpful"`
	}
}
