/*
|--------------------------------------------------------------------------
| Audience Requests
|--------------------------------------------------------------------------
|
| The inputs of saved audiences, previews and tests. A rule travels as
| plain JSON and is checked by the rule language itself; a save names the
| lock version the editor started from, so two editors never overwrite
| each other.
|
*/

package requests

import "encoding/json"

/*
AudienceRequest names a saved audience in the path.
*/
type AudienceRequest struct {
	Audience string `path:"audience"`
}

/*
AudienceBody is a saved audience's editable fields.
*/
type AudienceBody struct {
	Name        *string         `json:"name,omitempty"         maxLength:"80"`
	Description *string         `json:"description,omitempty"  maxLength:"500"`
	Rules       json.RawMessage `json:"rules,omitempty"`
	LockVersion int32           `json:"lock_version,omitempty"`
}

/*
StoreAudienceRequest is the input of POST /audiences.
*/
type StoreAudienceRequest struct {
	Body AudienceBody
}

/*
UpdateAudienceRequest is the input of PATCH /audiences/{audience}.
*/
type UpdateAudienceRequest struct {
	Audience string `path:"audience"`
	Body     AudienceBody
}
