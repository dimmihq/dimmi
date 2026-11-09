/*
|--------------------------------------------------------------------------
| Contact List Requests
|--------------------------------------------------------------------------
|
| The inputs of tagging customers in bulk: a CSV list arrives as a
| multipart form with the file in the file field and the tag's name in
| the tag field; freezing an audience names the tag in its body.
|
*/

package requests

import fhttp "github.com/dimmihq/dimmi/framework/http"

/*
StoreContactListRequest is the input of POST /contact-lists.
*/
type StoreContactListRequest struct {
	RawBody fhttp.MultipartForm
}

/*
Tag returns the tag's name the form gives.
*/
func (r *StoreContactListRequest) Tag() string {
	values := r.RawBody.Value["tag"]
	if len(values) == 0 {
		return ""
	}

	return values[0]
}

/*
FreezeAudienceRequest is the input of POST /audiences/{audience}/freezes.
*/
type FreezeAudienceRequest struct {
	Audience string `path:"audience"`
	Body     struct {
		Tag string `json:"tag" maxLength:"50"`
	}
}
