/*
|--------------------------------------------------------------------------
| Messenger And Instagram Requests
|--------------------------------------------------------------------------
|
| The inputs of the endpoints that connect and manage Facebook Pages and
| Instagram accounts.
|
*/

package requests

/*
SocialChannelRequest is the input of the endpoints that name a
connection.
*/
type SocialChannelRequest struct {
	Channel string `path:"channel"`
}

/*
StoreSocialSignupRequest is the input of POST /channels/{platform}/signups:
the code Meta's sign-in popup handed back and the address it returned
to.
*/
type StoreSocialSignupRequest struct {
	Body struct {
		Code        string `json:"code"                   maxLength:"2048" minLength:"1"`
		RedirectURI string `json:"redirect_uri,omitempty" maxLength:"2048"`
	}
}

/*
StoreSocialChannelRequest is the input of POST /channels/{platform}:
the sign-in's grant, the account picked from it, and the inbox its
conversations land in, the default one when left out.
*/
type StoreSocialChannelRequest struct {
	Body struct {
		Grant     string  `json:"grant"              maxLength:"128" minLength:"1"`
		AccountID string  `json:"account_id"         maxLength:"64"  minLength:"1"`
		InboxID   *string `json:"inbox_id,omitempty"`
	}
}

/*
UpdateSocialChannelRequest is the input of PATCH
/channels/{platform}/{channel}.
*/
type UpdateSocialChannelRequest struct {
	Channel string `path:"channel"`
	Body    struct {
		InboxID *string `json:"inbox_id,omitempty"`
	}
}

/*
SocialConversationRequest is the input of GET
/conversations/{conversation}/{platform}.
*/
type SocialConversationRequest struct {
	Conversation string `path:"conversation"`
}
