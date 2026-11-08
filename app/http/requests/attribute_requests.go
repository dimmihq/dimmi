/*
|--------------------------------------------------------------------------
| Attribute Definition Requests
|--------------------------------------------------------------------------
|
| The inputs of attribute definitions. A key and a type are given once,
| when the attribute is defined; afterwards only its words, list values,
| status and whether browsers may write it change.
|
*/

package requests

/*
AttributeRequest names a definition in the path.
*/
type AttributeRequest struct {
	Attribute string `path:"attribute"`
}

/*
ListAttributesRequest is the input of GET /attribute-definitions.
*/
type ListAttributesRequest struct {
	Object string `query:"filter[object]" enum:"contact,company,"`
}

/*
StoreAttributeRequest is the input of POST /attribute-definitions.
*/
type StoreAttributeRequest struct {
	Body struct {
		Object      string   `json:"object"                maxLength:"16"`
		Key         string   `json:"key"                   maxLength:"64"`
		Label       string   `json:"label"                 maxLength:"80"`
		Description *string  `json:"description,omitempty" maxLength:"500"`
		DataType    string   `json:"data_type"             maxLength:"16"`
		Options     []string `json:"options,omitempty"     maxItems:"200"`
	}
}

/*
UpdateAttributeRequest is the input of PATCH
/attribute-definitions/{attribute}.
*/
type UpdateAttributeRequest struct {
	Attribute string `path:"attribute"`
	Body      struct {
		Label          *string   `json:"label,omitempty"           maxLength:"80"`
		Description    *string   `json:"description,omitempty"     maxLength:"500"`
		Options        *[]string `json:"options,omitempty"         maxItems:"200"`
		Status         *string   `json:"status,omitempty"          enum:"active,archived"`
		ClientWritable *bool     `json:"client_writable,omitempty"`
	}
}
