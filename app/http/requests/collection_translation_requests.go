/*
|--------------------------------------------------------------------------
| Collection Translation Requests
|--------------------------------------------------------------------------
|
| The inputs of a collection's translations, named by the collection and
| the language.
|
*/

package requests

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
