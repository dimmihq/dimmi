/*
|--------------------------------------------------------------------------
| Identity Verification Requests
|--------------------------------------------------------------------------
|
| The inputs of a workspace's visitor sign-in settings and its counts.
|
*/

package requests

/*
UpdateIdentitySettingsBody is the body of PATCH
/identity-verification/settings. Force switches to required past the
rollout guard.
*/
type UpdateIdentitySettingsBody struct {
	Mode                     *string   `json:"mode,omitempty"                       enum:"off,optional,required"`
	Force                    bool      `json:"force,omitempty"`
	ClientWritableAttributes *[]string `json:"client_writable_attributes,omitempty" maxItems:"20"`
}

/*
UpdateIdentitySettingsRequest is the input of PATCH
/identity-verification/settings.
*/
type UpdateIdentitySettingsRequest struct {
	Body UpdateIdentitySettingsBody
}

/*
IdentityStatsRequest is the input of GET /identity-verification/stats:
how many days back, 7 when left out.
*/
type IdentityStatsRequest struct {
	Days int `query:"days" minimum:"0" maximum:"90"`
}
