/*
|--------------------------------------------------------------------------
| Streamable HTTP Transport
|--------------------------------------------------------------------------
|
| Remote agents reach the server with one POST per message and keep no
| session: every request carries its own credential, so any process can
| answer it and a revoked token stops working on the next call.
|
| A request without a valid credential gets 401 with a WWW-Authenticate
| header naming the protected resource metadata, which tells the client
| which authorization server issues tokens for this server. That is how
| a client that has never been set up finds out how to sign in. Browsers
| on other sites are refused, so a page cannot drive a local server
| through the person's browser.
|
*/

package mcp

import (
	"errors"
	"fmt"
	"strings"

	sdkauth "github.com/modelcontextprotocol/go-sdk/auth"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"

	"github.com/dimmihq/dimmi/framework/auth"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/origin"
	"github.com/dimmihq/dimmi/framework/tenancy"
)

/*
ErrOriginRefused is the fault for a browser request from a site that is
not allowed to call the server.
*/
var ErrOriginRefused = fault.New(fault.Forbidden, "origin_not_allowed", "This origin may not call this server.")

/*
Options configure the HTTP handler.

MaxBody caps a request body in bytes; zero keeps the protocol library's
default. Auth authenticates every request and answers a refused one
with the challenge; without it the caller must already be in the
request context, put there by the route's auth middleware, which sends
no challenge. ResourceMetadata is the URL of the protected resource
metadata named in 401 responses. AllowedOrigins lists the origin
patterns browsers may call from, such as "https://app.example.com";
requests without an Origin header, from programs, are always let in.
*/
type Options struct {
	MaxBody          int64
	Auth             auth.Guard
	ResourceMetadata string
	AllowedOrigins   []string
}

/*
ResourceMetadata describes the server as an OAuth protected resource.

Resource is the server's URL as tokens name it, AuthorizationServer the
issuer URL of the server that grants tokens for it, Scopes the scopes
it understands and Name a name for people.
*/
type ResourceMetadata struct {
	Resource            string
	AuthorizationServer string
	Scopes              []string
	Name                string
}

/*
HTTPHandler returns the handler serving the server over streamable HTTP
without sessions, answering every message with JSON. The "toolsets"
query parameter, a comma separated list, enables toolsets for the
request.
*/
func (s *Server) HTTPHandler(opts Options) http.Handler {
	streamable := sdk.NewStreamableHTTPHandler(
		func(*http.Request) *sdk.Server { return s.sdk },
		&sdk.StreamableHTTPOptions{Stateless: true, JSONResponse: true, MaxRequestBodyBytes: opts.MaxBody},
	)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := checkOrigin(r, opts.AllowedOrigins); err != nil {
			s.errors.Render(w, r, err)

			return
		}

		p, err := authenticate(r, opts.Auth)
		if err != nil {
			s.challenge(w, r, opts.ResourceMetadata, err)

			return
		}

		ctx := http.WithPrincipal(r.Context(), p)
		if tenant := p.Tenant(); tenant != "" {
			ctx = tenancy.With(ctx, tenant)
		}

		ctx, err = s.WithToolsets(ctx, toolsets(r)...)
		if err != nil {
			s.errors.Render(w, r, err)

			return
		}

		streamable.ServeHTTP(w, r.WithContext(ctx))
	})
}

/*
ProtectedResourceHandler returns the handler serving the protected
resource metadata document, which clients fetch without credentials to
find the authorization server.
*/
func ProtectedResourceHandler(m ResourceMetadata) http.Handler {
	return sdkauth.ProtectedResourceMetadataHandler(&oauthex.ProtectedResourceMetadata{
		Resource:               m.Resource,
		AuthorizationServers:   []string{m.AuthorizationServer},
		ScopesSupported:        m.Scopes,
		BearerMethodsSupported: []string{"header"},
		ResourceName:           m.Name,
	})
}

/*
checkOrigin refuses a request whose Origin header matches none of the
allowed patterns.
*/
func checkOrigin(r *http.Request, allowed []string) error {
	header := r.Header.Get("Origin")
	if header == "" {
		return nil
	}

	o, err := origin.Parse(header)
	if err != nil {
		return ErrOriginRefused
	}

	for _, pattern := range allowed {
		if origin.Match(pattern, o) {
			return nil
		}
	}

	return ErrOriginRefused
}

/*
authenticate returns the caller of the request, from the guard when
there is one and from the request context otherwise.
*/
func authenticate(r *http.Request, guard auth.Guard) (auth.Principal, error) {
	if guard == nil {
		p, ok := auth.From(r.Context())
		if !ok {
			return nil, auth.ErrUnauthenticated
		}

		return p, nil
	}

	p, err := guard.Authenticate(r)
	if errors.Is(err, auth.ErrNoCredentials) {
		return nil, auth.ErrUnauthenticated
	}

	if err != nil {
		return nil, fmt.Errorf("authenticate mcp request: %w", err)
	}

	return p, nil
}

/*
challenge renders an authentication failure. A 401 names the protected
resource metadata, and says when the token sent was refused, so the
client knows to sign in again rather than for the first time.
*/
func (s *Server) challenge(w http.ResponseWriter, r *http.Request, metadata string, err error) {
	if fault.KindOf(err) == fault.Unauthenticated {
		params := []string{}
		if errors.Is(err, auth.ErrInvalidToken) {
			params = append(params, `error="invalid_token"`)
		}

		if metadata != "" {
			params = append(params, fmt.Sprintf("resource_metadata=%q", metadata))
		}

		w.Header().Set("WWW-Authenticate", strings.TrimSpace("Bearer "+strings.Join(params, ", ")))
	}

	s.errors.Render(w, r, err)
}

/*
toolsets returns the toolsets the request's query enables.
*/
func toolsets(r *http.Request) []string {
	var names []string

	for name := range strings.SplitSeq(r.URL.Query().Get("toolsets"), ",") {
		if name = strings.TrimSpace(name); name != "" {
			names = append(names, name)
		}
	}

	return names
}
