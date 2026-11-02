/*
|--------------------------------------------------------------------------
| HTTP Client
|--------------------------------------------------------------------------
|
| Every outbound request leaves through this client, whether the
| application builds it or a provider SDK does. One place then enforces
| what must stay true for all of them: connections reach only public
| addresses unless the configuration allows a host, only http and https
| are spoken, redirects are few and checked on every hop, responses are
| size capped, and requests time out.
|
| No proxy from the environment is used, since a proxy would make the
| connection checks see the proxy instead of the real destination, and
| no cookies are kept, since requests made for different tenants must
| never share state.
|
| Tests replace the transport in place with a fake, so every holder of
| the client, SDKs included, stops reaching the network at once.
|
*/

package httpclient

import (
	"cmp"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"resty.dev/v3"

	"github.com/dimmihq/dimmi/framework/httpclient/safedial"
)

/*
Errors a request returns.

ErrBlockedHost refuses a destination: a private address, a scheme other
than http or https, or a redirect to either. ErrTooLarge means the
response body exceeds the size cap. ErrTooManyRedirects stops a redirect
chain past the configured length.
*/
var (
	ErrBlockedHost      = errors.New("httpclient: host not allowed")
	ErrTooLarge         = errors.New("httpclient: response body too large")
	ErrTooManyRedirects = errors.New("httpclient: too many redirects")
)

/*
ErrFakesNotAllowed is what ReplaceTransport panics with on a client
whose configuration does not allow fakes.
*/
var ErrFakesNotAllowed = errors.New("httpclient: fakes are allowed only in tests")

/*
Defaults for every zero field of Config.
*/
const (
	DefaultTimeout      = 30 * time.Second
	DefaultMaxBody      = 10 << 20
	DefaultMaxRedirects = 3
)

/*
Config configures the client.

AllowPrivateNetworks lets requests reach private and loopback addresses;
AllowHosts is the narrower choice, listing host:port pairs such as
localhost:11434. Timeout bounds a whole request including its body,
MaxBody caps a response body in bytes, and MaxRedirects caps how many
redirects one request follows. AllowFakes, set in code and only when
the application runs its tests, lets a fake replace the transport.
*/
type Config struct {
	AllowPrivateNetworks bool          `koanf:"allow_private_networks"`
	AllowHosts           []string      `koanf:"allow_hosts"`
	Timeout              time.Duration `default:"30s"      koanf:"timeout"`
	MaxBody              int64         `default:"10485760" koanf:"max_body"`
	MaxRedirects         int           `default:"3"        koanf:"max_redirects"`
	AllowFakes           bool          `koanf:"-"`
}

/*
Client makes outbound requests. It is safe for concurrent use.
*/
type Client struct {
	cfg       Config
	dialer    *safedial.Dialer
	transport *switchable
	follow    *resty.Client
	stay      *resty.Client
	std       *http.Client
}

/*
New returns a client for the configuration.
*/
func New(cfg Config) (*Client, error) {
	cfg.Timeout = cmp.Or(cfg.Timeout, DefaultTimeout)
	cfg.MaxBody = cmp.Or(cfg.MaxBody, DefaultMaxBody)
	cfg.MaxRedirects = cmp.Or(cfg.MaxRedirects, DefaultMaxRedirects)

	dialer, err := safedial.New(safedial.Options{
		AllowPrivateNetworks: cfg.AllowPrivateNetworks,
		AllowHosts:           cfg.AllowHosts,
	})
	if err != nil {
		return nil, fmt.Errorf("http client: %w", err)
	}

	c := &Client{cfg: cfg, dialer: dialer}
	c.transport = &switchable{current: &http.Transport{
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
	}}
	c.std = &http.Client{Transport: c.transport, CheckRedirect: c.checkRedirect}
	c.follow = newResty(&http.Client{Transport: c.transport, CheckRedirect: c.checkRedirect})
	c.stay = newResty(&http.Client{Transport: c.transport, CheckRedirect: stayOnFirstResponse})

	return c, nil
}

/*
New starts a request with the client's timeout, size cap and redirect
policy.
*/
func (c *Client) New() *Request {
	return &Request{
		client:    c,
		header:    http.Header{},
		timeout:   c.cfg.Timeout,
		maxBody:   c.cfg.MaxBody,
		redirects: true,
	}
}

/*
StdClient returns a standard library client for SDKs that take one. It
shares the connection checks, redirect policy and fakes of this client.
It sets no overall timeout and no size cap, because SDKs stream long
responses; they bound requests through their contexts.
*/
func (c *Client) StdClient() *http.Client {
	return c.std
}

/*
ReplaceTransport swaps the transport every request leaves through, in
place, for the fakes of httpclienttest. It panics with
ErrFakesNotAllowed unless the configuration allows fakes: a fake
outside tests would answer real requests with canned responses, which
is a programming error.
*/
func (c *Client) ReplaceTransport(rt http.RoundTripper) {
	if !c.cfg.AllowFakes {
		panic(ErrFakesNotAllowed)
	}

	c.transport.swap(rt)
}

/*
CheckAddress refuses a host:port the client may not reach, without
connecting, so a fake keeps refusing private addresses written as IP
literals or localhost as the real transport would.
*/
func (c *Client) CheckAddress(address string) error {
	if err := c.dialer.Check(address); err != nil {
		return fmt.Errorf("check %s: %w", address, err)
	}

	return nil
}

/*
checkRedirect follows a redirect only to http or https and only up to
the configured number of hops. Every hop dials through the same checked
transport.
*/
func (c *Client) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) > c.cfg.MaxRedirects {
		return fmt.Errorf("%w: more than %d", ErrTooManyRedirects, c.cfg.MaxRedirects)
	}

	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		return fmt.Errorf("%w: redirect to %s scheme", ErrBlockedHost, req.URL.Scheme)
	}

	return nil
}

func stayOnFirstResponse(*http.Request, []*http.Request) error {
	return http.ErrUseLastResponse
}

/*
newResty wraps the standard client in resty with its logging silenced;
errors reach the caller instead.
*/
func newResty(hc *http.Client) *resty.Client {
	return resty.NewWithClient(hc).SetLogger(silent{})
}

/*
switchable is a transport that can be replaced while in use.
*/
type switchable struct {
	mu      sync.RWMutex
	current http.RoundTripper
}

/*
RoundTrip sends the request on the current transport.
*/
func (s *switchable) RoundTrip(req *http.Request) (*http.Response, error) {
	s.mu.RLock()
	current := s.current
	s.mu.RUnlock()

	return current.RoundTrip(req)
}

func (s *switchable) swap(rt http.RoundTripper) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.current = rt
}

/*
silent discards resty's own log lines.
*/
type silent struct{}

/*
Errorf discards the line.
*/
func (silent) Errorf(string, ...any) {}

/*
Warnf discards the line.
*/
func (silent) Warnf(string, ...any) {}

/*
Debugf discards the line.
*/
func (silent) Debugf(string, ...any) {}
