/*
|--------------------------------------------------------------------------
| Tag Requests
|--------------------------------------------------------------------------
|
| Inputs of the endpoints on tags. A color is a hex value kept as the
| workspace wrote it. An update keeps the raw body to tell a color
| cleared with null from one left out.
|
*/

package requests

/*
TagRequest is the input of the endpoints that name only a tag.
*/
type TagRequest struct {
	Tag string `path:"tag"`
}

/*
StoreTagRequest is the input of POST /tags.
*/
type StoreTagRequest struct {
	Body struct {
		Name  string  `json:"name"            validate:"required,min=1,max=100"`
		Color *string `json:"color,omitempty" validate:"omitempty,hexcolor"`
	}
}

/*
UpdateTagRequest is the input of PATCH /tags/{tag}.
*/
type UpdateTagRequest struct {
	Tag  string `path:"tag"`
	Body struct {
		Name  *string `json:"name,omitempty"  validate:"omitempty,min=1,max=100"`
		Color *string `json:"color,omitempty" validate:"omitempty,hexcolor"`
	}
	RawBody []byte
}

/*
StoreConversationTagsRequest is the input of POST
/conversations/{conversation}/tags: tags by id or name, and the message
that prompted them when there is one.
*/
type StoreConversationTagsRequest struct {
	Conversation string `path:"conversation"`
	Body         struct {
		Tags      []string `json:"tags"                 validate:"required,min=1,max=50,dive,min=1,max=100"`
		MessageID *string  `json:"message_id,omitempty"`
	}
}

/*
ConversationTagRequest is the input of DELETE
/conversations/{conversation}/tags/{tag}.
*/
type ConversationTagRequest struct {
	Conversation string `path:"conversation"`
	Tag          string `path:"tag"`
}
