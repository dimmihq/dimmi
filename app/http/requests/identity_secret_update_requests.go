/*
|--------------------------------------------------------------------------
| Identity Secret Update Requests
|--------------------------------------------------------------------------
|
| A change to an existing secret: its name, or its status. Fields left
| out stay as they are.
|
*/

package requests

import "time"

/*
UpdateIdentitySecretBody is the body of PATCH /identity-secrets/{secret}:
a new name, status retiring with an optional date, or status revoked
with a reason and whether to keep the visitors it signed in.
*/
type UpdateIdentitySecretBody struct {
	Name           *string    `json:"name,omitempty"            maxLength:"60"`
	Status         *string    `json:"status,omitempty"          enum:"active,retiring,revoked"`
	RetiresAt      *time.Time `json:"retires_at,omitempty"`
	Reason         string     `json:"reason,omitempty"          enum:"manual,compromised"`
	RevokeSessions *bool      `json:"revoke_sessions,omitempty"`
}

/*
UpdateIdentitySecretRequest is the input of PATCH
/identity-secrets/{secret}.
*/
type UpdateIdentitySecretRequest struct {
	Secret string `path:"secret"`
	Body   UpdateIdentitySecretBody
}
