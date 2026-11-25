/*
|--------------------------------------------------------------------------
| AI Conversation Attribute Requests
|--------------------------------------------------------------------------
|
| The inputs of AI conversation attributes. A key and a type are given
| once, when the attribute is made; afterwards only its words, its
| choices and its status change.
|
*/

package requests

/*
AIAttributeRequest names an AI attribute in the path.
*/
type AIAttributeRequest struct {
	Attribute string `path:"attribute"`
}

/*
StoreAIAttributeRequest is the input of POST /ai/conversation-attributes.
*/
type StoreAIAttributeRequest struct {
	Body struct {
		Key         string   `json:"key"                   maxLength:"64"`
		Label       string   `json:"label"                 maxLength:"80"`
		Description string   `json:"description,omitempty" maxLength:"500"`
		DataType    string   `json:"data_type"             enum:"text,number,date,boolean,choice"`
		Options     []string `json:"options,omitempty"     maxItems:"30"`
	}
}

/*
UpdateAIAttributeRequest is the input of PATCH
/ai/conversation-attributes/{attribute}.
*/
type UpdateAIAttributeRequest struct {
	Attribute string `path:"attribute"`
	Body      struct {
		Label       *string   `json:"label,omitempty"       maxLength:"80"`
		Description *string   `json:"description,omitempty" maxLength:"500"`
		Options     *[]string `json:"options,omitempty"     maxItems:"30"`
		Status      *string   `json:"status,omitempty"      enum:"active,archived"`
	}
}
