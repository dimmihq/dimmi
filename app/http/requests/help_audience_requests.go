/*
|--------------------------------------------------------------------------
| Article Audience Requests
|--------------------------------------------------------------------------
|
| The inputs of keeping an article or a collection to a saved audience.
| A null audience opens it to everyone.
|
*/

package requests

/*
ArticleAudienceRequest is the input of PATCH /articles/{article}/audience.
*/
type ArticleAudienceRequest struct {
	Article string `path:"article"`
	Body    struct {
		AudienceID *string `json:"audience_id" maxLength:"64"`
	}
}

/*
CollectionAudienceRequest is the input of PATCH
/collections/{collection}/audience.
*/
type CollectionAudienceRequest struct {
	Collection string `path:"collection"`
	Body       struct {
		AudienceID *string `json:"audience_id" maxLength:"64"`
	}
}
