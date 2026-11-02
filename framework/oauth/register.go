/*
|--------------------------------------------------------------------------
| Dynamic Client Registration
|--------------------------------------------------------------------------
|
| Agents and tools that have never met the server register themselves
| before their first authorization request. Anyone can call the
| endpoint, so a registered client is marked dynamic, may ask for no
| scope the server does not advertise, and registrations are counted per
| address. The count lives in Valkey; when it cannot be checked the
| endpoint refuses rather than letting a flood through.
|
*/

package oauth

import (
	"cmp"
	"encoding/json"
	stdhttp "net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/log"
)

/*
The client authentication methods a client registers.
*/
const (
	authNone        = "none"
	authSecretBasic = "client_secret_basic"
	authSecretPost  = "client_secret_post"
)

/*
The application types a client registers.
*/
const (
	appNative = "native"
	appWeb    = "web"
)

/*
maxRegistrationBytes bounds a registration request.
*/
const maxRegistrationBytes = 16 << 10

/*
registrationRequest is the client metadata a registration sends. Fields
the server does not use are ignored.
*/
type registrationRequest struct {
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	ApplicationType         string   `json:"application_type"`
	Scope                   string   `json:"scope"`
}

/*
registrationResponse is the registered client as the registration
answer describes it.
*/
type registrationResponse struct {
	ClientID                string   `json:"client_id"`
	ClientSecret            string   `json:"client_secret,omitempty"`
	ClientIDIssuedAt        int64    `json:"client_id_issued_at"`
	ClientSecretExpiresAt   *int64   `json:"client_secret_expires_at,omitempty"`
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	ApplicationType         string   `json:"application_type"`
	Scope                   string   `json:"scope"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if err := s.throttle(w, r); err != nil {
		s.writeError(w, r, err)

		return
	}

	res, err := s.register(w, r)
	if err != nil {
		s.writeError(w, r, err)

		return
	}

	noStore(w)
	http.WriteJSON(w, http.StatusCreated, res)
}

/*
throttle counts the registration against the client's address and
refuses it past the limit, or while the count cannot be checked.
*/
func (s *Server) throttle(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	limit := s.opts.Registration

	res, err := s.limiter.Attempt(ctx, "oauth:register:"+http.ClientIP(r), limit.MaxAttempts, limit.Window)
	if err != nil {
		log.From(ctx).WarnContext(ctx, "client registration refused: limiter unavailable", log.Err(err))
		w.Header().Set("Retry-After", "5")

		return &Error{
			Code: CodeTemporarilyUnavailable, Description: "Registration is unavailable; try again soon.",
			Status: http.StatusServiceUnavailable,
		}
	}

	res.WriteHeaders(w.Header(), s.clock.Now())

	if !res.Allowed {
		return &Error{
			Code: CodeTemporarilyUnavailable, Description: "Too many registrations; try again later.",
			Status: http.StatusTooManyRequests,
		}
	}

	return nil
}

/*
register reads the client metadata and stores the client.
*/
func (s *Server) register(w http.ResponseWriter, r *http.Request) (*registrationResponse, error) {
	var in registrationRequest

	decoder := json.NewDecoder(stdhttp.MaxBytesReader(w, r.Body, maxRegistrationBytes))
	if err := decoder.Decode(&in); err != nil {
		return nil, refuse(CodeInvalidClientMetadata, "The body must be a JSON object of at most 16 KB.")
	}

	in.TokenEndpointAuthMethod = cmp.Or(in.TokenEndpointAuthMethod, authNone)
	in.ApplicationType = cmp.Or(in.ApplicationType, appNative)

	c, err := s.registeredClient(in)
	if err != nil {
		return nil, err
	}

	c, secret, err := s.CreateClient(r.Context(), c)
	if err != nil {
		return nil, err
	}

	res := &registrationResponse{
		ClientID: c.ID, ClientSecret: secret, ClientIDIssuedAt: c.CreatedAt.Unix(), ClientName: c.Name,
		RedirectURIs: c.RedirectURIs, GrantTypes: c.GrantTypes, ResponseTypes: []string{"code"},
		TokenEndpointAuthMethod: in.TokenEndpointAuthMethod, ApplicationType: in.ApplicationType,
		Scope: strings.Join(c.Scopes, " "),
	}

	if c.Confidential {
		res.ClientSecretExpiresAt = new(int64)
	}

	return res, nil
}

/*
registeredClient checks the metadata a registration sends and returns
the client it describes.
*/
func (s *Server) registeredClient(in registrationRequest) (Client, error) {
	scopes := parseScopes(in.Scope)

	switch {
	case !slices.Contains(authMethods(), in.TokenEndpointAuthMethod):
		return Client{}, refuse(CodeInvalidClientMetadata, "token_endpoint_auth_method is not supported.")
	case len(in.ResponseTypes) > 0 && !slices.Equal(in.ResponseTypes, []string{"code"}):
		return Client{}, refuse(CodeInvalidClientMetadata, "response_types may only be code.")
	case in.ApplicationType != appNative && in.ApplicationType != appWeb:
		return Client{}, refuse(CodeInvalidClientMetadata, "application_type must be native or web.")
	case in.ApplicationType == appWeb && slices.ContainsFunc(in.RedirectURIs, notPublicHTTPS):
		return Client{}, refuse(CodeInvalidRedirectURI, "A web client's redirect URIs must be public https.")
	case !subset(scopes, s.opts.Scopes):
		return Client{}, refuse(CodeInvalidClientMetadata, "scope names scopes the server does not offer.")
	}

	return Client{
		Name:         in.ClientName,
		RedirectURIs: in.RedirectURIs,
		Scopes:       scopes,
		GrantTypes:   in.GrantTypes,
		Confidential: in.TokenEndpointAuthMethod != authNone,
		Dynamic:      true,
	}, nil
}

/*
notPublicHTTPS reports whether the URI is anything but https on a host
other than the loopback interface.
*/
func notPublicHTTPS(uri string) bool {
	u, err := url.Parse(uri)

	return err != nil || u.Scheme != "https" || loopback(u.Hostname())
}
