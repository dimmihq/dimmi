/*
|--------------------------------------------------------------------------
| Invitation Delivery Requests
|--------------------------------------------------------------------------
|
| Sending an invitation again. Whoever sends it can have the fresh link
| mailed, or only handed back to pass on by hand; either way the link
| sent before stops working, since only the newest one is kept.
|
*/

package requests

/*
DeliverInvitationRequest is the input of POST
/invitations/{invitation}/deliveries. The body is optional: channel
email mails a fresh link, the default; link only returns it, to pass on
by hand.
*/
type DeliverInvitationRequest struct {
	Invitation string `path:"invitation"`
	Body       *struct {
		Channel string `json:"channel,omitempty" enum:"email,link"`
	}
}

/*
LinkOnly reports whether the body asks for the link alone, with no mail.
*/
func (r *DeliverInvitationRequest) LinkOnly() bool {
	return r.Body != nil && r.Body.Channel == "link"
}
