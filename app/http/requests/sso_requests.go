/*
|--------------------------------------------------------------------------
| Single Sign-On Requests
|--------------------------------------------------------------------------
|
| Inputs of the endpoints on a company's connections and domains. A
| connection and a domain are named by their public ids. A client secret
| is only ever written, never read back, and a domain is sent as typed;
| the action works out its canonical form.
|
*/

package requests

/*
ShowSSOConnectionRequest is the input of GET and DELETE
/sso-connections/{connection}.
*/
type ShowSSOConnectionRequest struct {
	Connection string `path:"connection"`
}

/*
StoreSSOConnectionRequest is the input of POST /sso-connections.
*/
type StoreSSOConnectionRequest struct {
	Body struct {
		Protocol     string    `json:"protocol"                validate:"required,oneof=saml oidc"`
		Name         string    `json:"name"                    validate:"required,max=80"`
		Config       SSOConfig `json:"config,omitzero"`
		ClientSecret string    `json:"client_secret,omitempty" validate:"max=2000"`
	}
}

/*
UpdateSSOConnectionRequest is the input of PATCH
/sso-connections/{connection}.
*/
type UpdateSSOConnectionRequest struct {
	Connection string `path:"connection"`
	Body       struct {
		Name         *string    `json:"name,omitempty"          validate:"omitempty,min=1,max=80"`
		Config       *SSOConfig `json:"config,omitempty"`
		ClientSecret *string    `json:"client_secret,omitempty" validate:"omitempty,max=2000"`
		Status       *string    `json:"status,omitempty"        validate:"omitempty,max=20"`
		Enforced     *bool      `json:"enforced,omitempty"`

		JITEnabled     *bool           `json:"jit_enabled,omitempty"`
		DefaultRole    *string         `json:"default_role,omitempty"   validate:"omitempty,oneof=admin member"`
		DefaultCustom  *string         `json:"default_custom_role_id,omitempty"`
		DefaultTeamIDs *[]string       `json:"default_team_ids,omitempty"`
		GroupRoleMap   *[]SSOGroupRole `json:"group_role_map,omitempty" validate:"omitempty,max=50,dive"`

		ConfirmOwnersCanSignIn bool `json:"confirm_owners_can_sign_in,omitempty"`
	}
	RawBody []byte
}
