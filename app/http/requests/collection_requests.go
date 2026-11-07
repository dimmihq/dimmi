/*
|--------------------------------------------------------------------------
| Collection Requests
|--------------------------------------------------------------------------
|
| The inputs of a help center's collections. Fields left out of a change
| stay as they are.
|
*/

package requests

/*
CollectionBody is a collection's fields.
*/
type CollectionBody struct {
	ParentID    *string `json:"parent_id,omitempty"   maxLength:"64"`
	Name        *string `json:"name,omitempty"        maxLength:"80"`
	Slug        *string `json:"slug,omitempty"        maxLength:"80"`
	Description *string `json:"description,omitempty" maxLength:"300"`
	Icon        *string `json:"icon,omitempty"        maxLength:"40"`
	Position    *int32  `json:"position,omitempty"`
}

/*
StoreCollectionRequest is the input of POST
/help-centers/{help_center}/collections.
*/
type StoreCollectionRequest struct {
	HelpCenter string `path:"help_center"`
	Body       CollectionBody
}

/*
CollectionRequest names a collection in the path.
*/
type CollectionRequest struct {
	Collection string `path:"collection"`
}

/*
UpdateCollectionRequest is the input of PATCH /collections/{collection};
an empty parent_id moves it to the top level.
*/
type UpdateCollectionRequest struct {
	Collection string `path:"collection"`
	Body       CollectionBody
}
