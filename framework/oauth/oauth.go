/*
|--------------------------------------------------------------------------
| Authorization Server
|--------------------------------------------------------------------------
|
| Other applications act for a user only with the user's consent, and
| only through tokens this server issued. Every secret it hands out, from
| authorization codes to client secrets, is shown once and stored as a
| digest, so a copy of the database grants nothing. Login and consent
| pages belong to the application: the server checks requests, issues and
| rotates tokens, and tells the bearer guard who a token stands for.
|
*/

package oauth

import (
	"cmp"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ratelimit"
)

/*
The guards the resolver names. A first-party grant reaches the user's
whole account and answers as GuardFirstParty; every other grant answers
as GuardThirdParty, held to its tenant and scopes.
*/
const (
	GuardFirstParty = "app"
	GuardThirdParty = "token"
)

/*
ErrInvalidOptions is returned by New for options the server cannot run
with, such as an issuer that is not an absolute URL.
*/
var ErrInvalidOptions = errors.New("oauth: invalid options")

var prefixPattern = regexp.MustCompile(`^[A-Za-z0-9_]*$`)

/*
Options configure the server.

Issuer is the server's URL, such as "https://example.com", with no path:
every endpoint lives under it. Prefix starts every code, token and
secret, so a leaked one is recognizable. Scopes are the scopes the
server advertises and lets self-registered clients ask for.
AuthorizationPath is where the application serves the authorization
page (default "/oauth/authorize").

CodeTTL bounds an authorization code (default one minute),
AccessTokenTTL an access token (default one hour) and
FirstPartyAccessTokenTTL a first-party one (default fifteen minutes).
RefreshTokenTTL is how long a refresh token may sit unused (default
ninety days); each refresh starts the window again.

Clients are clients the application configures rather than stores,
such as its own mobile app; they must be public. Errors reports
failures that are not OAuth errors.
*/
type Options struct {
	Issuer                   string        `koanf:"issuer"`
	Prefix                   string        `koanf:"prefix"`
	Scopes                   []string      `koanf:"scopes"`
	AuthorizationPath        string        `koanf:"authorization_path"`
	CodeTTL                  time.Duration `koanf:"code_ttl"`
	AccessTokenTTL           time.Duration `koanf:"access_token_ttl"`
	FirstPartyAccessTokenTTL time.Duration `koanf:"first_party_access_token_ttl"`
	RefreshTokenTTL          time.Duration `koanf:"refresh_token_ttl"`
	Registration             Registration  `koanf:"registration"`
	Clients                  []Client      `koanf:"-"`
	Errors                   http.Errors   `koanf:"-"`
}

/*
Registration configures dynamic client registration.

Enabled serves the registration endpoint and advertises it. MaxAttempts
registrations are allowed per client address in each Window (defaults
twenty per hour).
*/
type Registration struct {
	Enabled     bool          `koanf:"enabled"`
	MaxAttempts int           `koanf:"max_attempts"`
	Window      time.Duration `koanf:"window"`
}

/*
Server is the authorization server.
*/
type Server struct {
	db      *database.DB
	clock   clock.Clock
	limiter *ratelimit.Limiter
	opts    Options
}

/*
New returns a server on the database. The limiter counts registrations
and may be nil when registration is disabled.
*/
func New(db *database.DB, clk clock.Clock, limiter *ratelimit.Limiter, opts Options) (*Server, error) {
	opts, err := withDefaults(opts)
	if err != nil {
		return nil, err
	}

	if opts.Registration.Enabled && limiter == nil {
		return nil, fmt.Errorf("%w: registration needs a rate limiter", ErrInvalidOptions)
	}

	s := &Server{db: db, clock: clk, limiter: limiter, opts: opts}

	for i, c := range opts.Clients {
		if c.ID == "" || c.Confidential {
			return nil, fmt.Errorf("%w: configured client %q needs an id and must be public", ErrInvalidOptions, c.ID)
		}

		prepared, err := s.prepareClient(c)
		if err != nil {
			return nil, fmt.Errorf("%w: configured client %q: %w", ErrInvalidOptions, c.ID, err)
		}

		s.opts.Clients[i] = prepared
	}

	return s, nil
}

/*
Issuer returns the server's URL, which clients compare against the
issuer of the metadata and of every authorization response.
*/
func (s *Server) Issuer() string {
	return s.opts.Issuer
}

/*
withDefaults checks the options and fills in the defaults.
*/
func withDefaults(opts Options) (Options, error) {
	issuer, err := checkIssuer(opts.Issuer)
	if err != nil {
		return Options{}, err
	}

	if !prefixPattern.MatchString(opts.Prefix) {
		return Options{}, fmt.Errorf("%w: prefix %q must be letters, digits and underscores", ErrInvalidOptions,
			opts.Prefix)
	}

	opts.Issuer = issuer
	opts.Clients = append([]Client{}, opts.Clients...)
	opts.AuthorizationPath = cmp.Or(opts.AuthorizationPath, "/oauth/authorize")
	opts.CodeTTL = cmp.Or(opts.CodeTTL, time.Minute)
	opts.AccessTokenTTL = cmp.Or(opts.AccessTokenTTL, time.Hour)
	opts.FirstPartyAccessTokenTTL = cmp.Or(opts.FirstPartyAccessTokenTTL, 15*time.Minute)
	opts.RefreshTokenTTL = cmp.Or(opts.RefreshTokenTTL, 90*24*time.Hour)
	opts.Registration.MaxAttempts = cmp.Or(opts.Registration.MaxAttempts, 20)
	opts.Registration.Window = cmp.Or(opts.Registration.Window, time.Hour)

	return opts, nil
}

/*
checkIssuer returns the issuer without a trailing slash, or an error
when it is not an absolute http or https URL without path, query or
fragment.
*/
func checkIssuer(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("%w: issuer: %w", ErrInvalidOptions, err)
	}

	if (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || strings.Trim(u.Path, "/") != "" ||
		u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return "", fmt.Errorf("%w: issuer %q must be an http or https origin", ErrInvalidOptions, raw)
	}

	return u.Scheme + "://" + u.Host, nil
}
