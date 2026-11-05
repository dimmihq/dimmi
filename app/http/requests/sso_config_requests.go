/*
|--------------------------------------------------------------------------
| Single Sign-On Settings
|--------------------------------------------------------------------------
|
| The settings a connection is sent with: how to reach the identity
| provider, which attributes carry a person, and which groups get which
| role. SAML uses the identity provider fields, OpenID Connect the issuer,
| client and scopes.
|
*/

package requests

/*
SSOAttributeMap names the identity provider attributes of a person.
*/
type SSOAttributeMap struct {
	Email       string `json:"email,omitempty"        validate:"max=200"`
	FirstName   string `json:"first_name,omitempty"   validate:"max=200"`
	LastName    string `json:"last_name,omitempty"    validate:"max=200"`
	DisplayName string `json:"display_name,omitempty" validate:"max=200"`
	Groups      string `json:"groups,omitempty"       validate:"max=200"`
}

/*
SSOConfig is a connection's settings as a request sends them.
*/
type SSOConfig struct {
	IDPEntityID     string          `json:"idp_entity_id,omitempty"    validate:"max=500"`
	IDPSSOURL       string          `json:"idp_sso_url,omitempty"      validate:"omitempty,url,max=2000"`
	IDPCertificates []string        `json:"idp_certificates,omitempty" validate:"max=5,dive,max=10000"`
	IDPMetadataURL  string          `json:"idp_metadata_url,omitempty" validate:"omitempty,url,max=2000"`
	NameIDFormat    string          `json:"name_id_format,omitempty"   validate:"max=200"`
	Issuer          string          `json:"issuer,omitempty"           validate:"omitempty,url,max=2000"`
	ClientID        string          `json:"client_id,omitempty"        validate:"max=500"`
	Scopes          []string        `json:"scopes,omitempty"           validate:"max=20,dive,max=100"`
	AttributeMap    SSOAttributeMap `json:"attribute_map,omitzero"`
}

/*
SSOGroupRole gives an identity provider group a role.
*/
type SSOGroupRole struct {
	Group        string  `json:"group"                    validate:"required,max=200"`
	Role         string  `json:"role,omitempty"           validate:"omitempty,oneof=admin member"`
	CustomRoleID *string `json:"custom_role_id,omitempty"`
}
