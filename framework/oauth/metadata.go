/*
|--------------------------------------------------------------------------
| Server Metadata and Routes
|--------------------------------------------------------------------------
|
| A client that knows only the issuer finds everything else in the
| metadata document: where to send users, where to spend codes, which
| scopes exist and that PKCE S256 is required. Clients that will not
| start a flow without that last fact refuse servers that leave it out,
| so it is always there. Protected resources point clients at the issuer,
| so the document is built here once for the endpoint and for anyone who
| needs to describe the server.
|
*/

package oauth

import (
	"slices"

	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/routing"
)

/*
The paths the server's endpoints are served at, under the issuer. The
exchange path is the token endpoint.
*/
const (
	pathMetadata = "/.well-known/oauth-authorization-server"
	pathExchange = "/oauth/token"
	pathRevoke   = "/oauth/revoke"
	pathRegister = "/oauth/register"
)

/*
Metadata is the server's metadata document. Its fields are those of the
authorization server metadata standard, under their standard names.
*/
type Metadata struct {
	Issuer                                     string   `json:"issuer"`
	AuthorizationEndpoint                      string   `json:"authorization_endpoint"`
	TokenEndpoint                              string   `json:"token_endpoint"`
	RevocationEndpoint                         string   `json:"revocation_endpoint"`
	RegistrationEndpoint                       string   `json:"registration_endpoint,omitempty"`
	ScopesSupported                            []string `json:"scopes_supported,omitempty"`
	ResponseTypesSupported                     []string `json:"response_types_supported"`
	ResponseModesSupported                     []string `json:"response_modes_supported"`
	GrantTypesSupported                        []string `json:"grant_types_supported"`
	TokenEndpointAuthMethodsSupported          []string `json:"token_endpoint_auth_methods_supported"`
	RevocationEndpointAuthMethodsSupported     []string `json:"revocation_endpoint_auth_methods_supported"`
	CodeChallengeMethodsSupported              []string `json:"code_challenge_methods_supported"`
	AuthorizationResponseIssParameterSupported bool     `json:"authorization_response_iss_parameter_supported"`
}

/*
Metadata returns the server's metadata document.
*/
func (s *Server) Metadata() Metadata {
	issuer := s.opts.Issuer
	m := Metadata{
		Issuer:                                     issuer,
		AuthorizationEndpoint:                      issuer + s.opts.AuthorizationPath,
		TokenEndpoint:                              issuer + pathExchange,
		RevocationEndpoint:                         issuer + pathRevoke,
		ScopesSupported:                            slices.Clone(s.opts.Scopes),
		ResponseTypesSupported:                     []string{"code"},
		ResponseModesSupported:                     []string{"query"},
		GrantTypesSupported:                        grantTypes(),
		TokenEndpointAuthMethodsSupported:          authMethods(),
		RevocationEndpointAuthMethodsSupported:     authMethods(),
		CodeChallengeMethodsSupported:              []string{methodS256},
		AuthorizationResponseIssParameterSupported: true,
	}

	if s.opts.Registration.Enabled {
		m.RegistrationEndpoint = issuer + pathRegister
	}

	return m
}

/*
MetadataURL returns where the metadata document is served.
*/
func (s *Server) MetadataURL() string {
	return s.opts.Issuer + pathMetadata
}

/*
Routes declares the metadata, token and revocation endpoints, and the
registration endpoint when registration is enabled, on the router. The
router must serve the issuer's root, so the paths match the metadata.
The authorization page is the application's own route.
*/
func (s *Server) Routes(r *routing.Router) {
	r.Handle(http.MethodGet, pathMetadata, "oauth.metadata", http.HandlerFunc(s.handleMetadata))
	r.Handle(http.MethodPost, pathExchange, "oauth.token", http.HandlerFunc(s.handleToken))
	r.Handle(http.MethodPost, pathRevoke, "oauth.revoke", http.HandlerFunc(s.handleRevoke))

	if s.opts.Registration.Enabled {
		r.Handle(http.MethodPost, pathRegister, "oauth.register", http.HandlerFunc(s.handleRegister))
	}
}

func (s *Server) handleMetadata(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=3600")
	http.WriteJSON(w, http.StatusOK, s.Metadata())
}

/*
authMethods lists the ways a client may authenticate: none for public
clients, and a secret in Basic or in the form for confidential ones.
*/
func authMethods() []string {
	return []string{authNone, authSecretBasic, authSecretPost}
}
