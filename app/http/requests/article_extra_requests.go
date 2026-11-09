/*
|--------------------------------------------------------------------------
| Article Extra Requests
|--------------------------------------------------------------------------
|
| The inputs of restoring a kept version of an article and of keeping a
| picture to place in one.
|
*/

package requests

import fhttp "github.com/dimmihq/dimmi/framework/http"

/*
RestoreArticleRequest is the input of POST
/articles/{article}/restorations: the version to put back.
*/
type RestoreArticleRequest struct {
	Article string `path:"article"`
	Body    struct {
		VersionID string `json:"version_id" maxLength:"64" minLength:"1"`
	}
}

/*
StoreHelpImageRequest is the input of POST /help-images: a multipart
form with the picture in the image field.
*/
type StoreHelpImageRequest struct {
	RawBody fhttp.MultipartForm
}
