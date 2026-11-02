/*
|--------------------------------------------------------------------------
| Clients
|--------------------------------------------------------------------------
|
| A client is an application allowed to ask users for access. What it
| registered bounds every request it makes: authorization answers go
| only to a redirect URI it registered, it asks for no scope beyond its
| list and uses no grant it did not register. A confidential client also
| proves itself with a secret, of which only a digest is kept; a public
| client, such as a mobile or command-line app, has nothing to keep a
| secret in and relies on PKCE alone.
|
*/

package oauth

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/support/str"
)

/*
The grant types clients register.
*/
const (
	GrantAuthorizationCode = "authorization_code"
	GrantRefreshToken      = "refresh_token"
)

/*
Limits on what a client registers.
*/
const (
	maxNameLength   = 100
	maxRedirectURIs = 10
)

/*
ErrClientExists is returned by CreateClient for an id that is taken.
*/
var ErrClientExists = errors.New("oauth: client id is taken")

/*
Client is an application registered to ask users for access.

ID names it in requests and Name is what users see on the consent
screen. RedirectURIs are the only addresses authorization answers go to,
matched exactly. Scopes are the most it may ask for; empty means every
scope the server supports. GrantTypes are the grants it may use; empty
means the authorization code and refresh token grants. A Confidential
client authenticates with a secret at the token endpoint. FirstParty
marks the application's own apps, whose grants reach the user's whole
account rather than one tenant. Dynamic marks a client that registered
itself, which a consent screen should say it cannot vouch for.
*/
type Client struct {
	ID           string
	Name         string
	RedirectURIs []string
	Scopes       []string
	GrantTypes   []string
	Confidential bool
	FirstParty   bool
	Dynamic      bool
	CreatedAt    time.Time
	secretHash   []byte
}

/*
CreateClient registers the client and returns it as stored with its
plain secret, which is never available again and is empty for a public
client. An empty ID gets a generated one. Metadata the server cannot
accept is an Error with code invalid_redirect_uri or
invalid_client_metadata.
*/
func (s *Server) CreateClient(ctx context.Context, c Client) (Client, string, error) {
	c, err := s.prepareClient(c)
	if err != nil {
		return Client{}, "", err
	}

	c.ID = cmp.Or(c.ID, s.opts.Prefix+kindClientID+str.Random(clientIDLength))
	c.CreatedAt = s.clock.Now()
	c.secretHash = nil

	var secret string
	if c.Confidential {
		secret = s.newSecret(kindClientSecret)
		c.secretHash = digest(secret)
	}

	sql := "INSERT INTO oauth_clients (id, secret_hash, name, redirect_uris, scopes, grant_types, first_party, " +
		"dynamic, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)"

	_, err = s.db.Exec(ctx, sql, c.ID, c.secretHash, c.Name, c.RedirectURIs, c.Scopes, c.GrantTypes, c.FirstParty,
		c.Dynamic, c.CreatedAt)
	if database.IsUniqueViolation(err, "oauth_clients_pkey") {
		return Client{}, "", fmt.Errorf("%w: %q", ErrClientExists, c.ID)
	}

	if err != nil {
		return Client{}, "", fmt.Errorf("insert client: %w", err)
	}

	return c, secret, nil
}

/*
prepareClient fills in the defaults and checks what the client
registers.
*/
func (s *Server) prepareClient(c Client) (Client, error) {
	c.Name = strings.TrimSpace(c.Name)
	c.RedirectURIs = slices.Clone(c.RedirectURIs)
	c.Scopes = slices.Clone(c.Scopes)
	c.GrantTypes = slices.Clone(c.GrantTypes)

	if len(c.Scopes) == 0 {
		c.Scopes = slices.Clone(s.opts.Scopes)
	}

	if len(c.GrantTypes) == 0 {
		c.GrantTypes = grantTypes()
	}

	switch {
	case c.Name == "" || utf8.RuneCountInString(c.Name) > maxNameLength:
		return Client{}, refuse(CodeInvalidClientMetadata, "client_name must be 1 to 100 characters.")
	case !slices.Contains(c.GrantTypes, GrantAuthorizationCode) || !subset(c.GrantTypes, grantTypes()):
		return Client{}, refuse(CodeInvalidClientMetadata,
			"grant_types must include authorization_code and may add refresh_token.")
	case len(c.RedirectURIs) == 0 || len(c.RedirectURIs) > maxRedirectURIs:
		return Client{}, refuse(CodeInvalidRedirectURI, "redirect_uris must list 1 to 10 URIs.")
	}

	for _, uri := range c.RedirectURIs {
		if err := checkRedirect(uri); err != nil {
			return Client{}, err
		}
	}

	return c, nil
}

/*
grantTypes lists the grant types the server supports.
*/
func grantTypes() []string {
	return []string{GrantAuthorizationCode, GrantRefreshToken}
}

/*
findClient returns the configured or stored client with the id. An
unknown client is an invalid_client Error.
*/
func (s *Server) findClient(ctx context.Context, id string) (Client, error) {
	if id == "" {
		return Client{}, errInvalidClient()
	}

	for _, c := range s.opts.Clients {
		if c.ID == id {
			return c, nil
		}
	}

	sql := "SELECT id, secret_hash, name, redirect_uris, scopes, grant_types, first_party, dynamic, created_at " +
		"FROM oauth_clients WHERE id = $1"

	c, err := scanClient(s.db.QueryRow(ctx, sql, id))
	if errors.Is(err, database.ErrNotFound) {
		return Client{}, errInvalidClient()
	}

	if err != nil {
		return Client{}, fmt.Errorf("find client: %w", err)
	}

	return c, nil
}

func scanClient(row pgx.Row) (Client, error) {
	var c Client

	err := row.Scan(&c.ID, &c.secretHash, &c.Name, &c.RedirectURIs, &c.Scopes, &c.GrantTypes, &c.FirstParty,
		&c.Dynamic, &c.CreatedAt)
	if err != nil {
		return Client{}, err
	}

	c.Confidential = c.secretHash != nil

	return c, nil
}
