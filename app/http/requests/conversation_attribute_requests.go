/*
|--------------------------------------------------------------------------
| Conversation Attribute Requests
|--------------------------------------------------------------------------
|
| The inputs of conversation attributes. A key and a type are given
| once, when the attribute is made; afterwards its words, choices,
| settings and status change. A conversation's values are set by key,
| a null clearing one.
|
*/

package requests

/*
StoreConversationAttributeRequest is the input of POST
/conversation-attributes.
*/
type StoreConversationAttributeRequest struct {
	Body struct {
		Key             string   `json:"key"                         maxLength:"64"`
		Label           string   `json:"label"                       maxLength:"80"`
		Description     string   `json:"description,omitempty"       maxLength:"500"`
		DataType        string   `json:"data_type"                   enum:"text,long_text,number,decimal,choice,multiple_choice,boolean,date,datetime,file,contact,company"` //nolint:lll // one enum
		Options         []string `json:"options,omitempty"           maxItems:"200"`
		FilledBy        string   `json:"filled_by,omitempty"         enum:"ai,teammates,both"`
		RequiredToClose bool     `json:"required_to_close,omitempty"`
		TeamIDs         []string `json:"team_ids,omitempty"          maxItems:"100"`
	}
}

/*
UpdateConversationAttributeRequest is the input of PATCH
/conversation-attributes/{attribute}.
*/
type UpdateConversationAttributeRequest struct {
	Attribute string `path:"attribute"`
	Body      struct {
		Label           *string   `json:"label,omitempty"             maxLength:"80"`
		Description     *string   `json:"description,omitempty"       maxLength:"500"`
		Options         *[]string `json:"options,omitempty"           maxItems:"200"`
		Status          *string   `json:"status,omitempty"            enum:"active,archived"`
		FilledBy        *string   `json:"filled_by,omitempty"         enum:"ai,teammates,both"`
		RequiredToClose *bool     `json:"required_to_close,omitempty"`
		TeamIDs         *[]string `json:"team_ids,omitempty"          maxItems:"100"`
	}
}

/*
UpdateConversationAttributeValuesRequest is the input of PATCH
/conversations/{conversation}/attributes.
*/
type UpdateConversationAttributeValuesRequest struct {
	Conversation string `path:"conversation"`
	Body         struct {
		Values map[string]any `json:"values" maxProperties:"100"`
	}
}
