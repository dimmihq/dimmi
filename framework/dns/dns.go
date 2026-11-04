/*
|--------------------------------------------------------------------------
| DNS Lookups
|--------------------------------------------------------------------------
|
| Domain checks read DNS: verification tokens in TXT records, mail
| routing in MX, custom domains pointing here through CNAME. A lookup
| crosses the network to servers the application does not run, so every
| one is bounded by a timeout, and tests swap in a fake zone instead of
| depending on the internet.
|
| Names are absolute: a name is never tried against the local search
| domains. Results come back without the trailing root dot, and an
| absent name or record type is always ErrNotFound.
|
*/

package dns

import (
	"context"
	"errors"
	"net/netip"
	"sync"
	"time"
)

/*
ErrNotFound means the name does not exist or has no record of the type
asked for.
*/
var ErrNotFound = errors.New("dns: no such record")

/*
ErrFakesNotAllowed is what Fake panics with on a client whose
configuration does not allow fakes.
*/
var ErrFakesNotAllowed = errors.New("dns: fakes are allowed only in tests")

/*
DefaultTimeout bounds a lookup when the configuration sets none.
*/
const DefaultTimeout = 5 * time.Second

/*
Config configures lookups.

Timeout bounds each lookup. Nameserver is the host:port of the server to
ask; empty uses the operating system's resolver. AllowFakes, set in code
and only when the application runs its tests, lets Fake replace the
resolver.
*/
type Config struct {
	Timeout    time.Duration `default:"5s" koanf:"timeout"`
	Nameserver string        `koanf:"nameserver"`
	AllowFakes bool          `koanf:"-"`
}

/*
MX is one mail exchanger: its host and preference, lower first.
*/
type MX struct {
	Host string
	Pref uint16
}

/*
Resolver looks up records by name.
*/
type Resolver interface {
	/*
		TXT returns the text of every TXT record, each record's strings
		joined.
	*/
	TXT(ctx context.Context, name string) ([]string, error)

	/*
		MX returns the mail exchangers, lowest preference first.
	*/
	MX(ctx context.Context, name string) ([]MX, error)

	/*
		CNAME returns the canonical name, following aliases. A name that is
		not an alias is its own canonical name.
	*/
	CNAME(ctx context.Context, name string) (string, error)

	/*
		Addrs returns the IPv4 and IPv6 addresses, following aliases.
	*/
	Addrs(ctx context.Context, name string) ([]netip.Addr, error)

	/*
		NS returns the name servers the name delegates to.
	*/
	NS(ctx context.Context, name string) ([]string, error)
}

/*
Client is the application's resolver. It asks real servers until a test
swaps in a fake, in place, so code holding the client sees the fake too.
*/
type Client struct {
	allowFakes bool

	mu       sync.RWMutex
	resolver Resolver
}

/*
New returns a client that asks real servers.
*/
func New(cfg Config) *Client {
	return &Client{allowFakes: cfg.AllowFakes, resolver: newSystem(cfg)}
}

/*
TXT returns the TXT records of the name.
*/
func (c *Client) TXT(ctx context.Context, name string) ([]string, error) {
	return c.current().TXT(ctx, name)
}

/*
MX returns the mail exchangers of the name.
*/
func (c *Client) MX(ctx context.Context, name string) ([]MX, error) {
	return c.current().MX(ctx, name)
}

/*
CNAME returns the canonical name of the name.
*/
func (c *Client) CNAME(ctx context.Context, name string) (string, error) {
	return c.current().CNAME(ctx, name)
}

/*
NS returns the name servers of the name.
*/
func (c *Client) NS(ctx context.Context, name string) ([]string, error) {
	return c.current().NS(ctx, name)
}

/*
Addrs returns the addresses of the name.
*/
func (c *Client) Addrs(ctx context.Context, name string) ([]netip.Addr, error) {
	return c.current().Addrs(ctx, name)
}

/*
Fake replaces the resolver with an empty fake zone and returns it.
It panics with ErrFakesNotAllowed unless the configuration allows fakes:
a fake outside tests would answer real lookups from an empty zone, which
is a programming error.
*/
func (c *Client) Fake() *Fake {
	if !c.allowFakes {
		panic(ErrFakesNotAllowed)
	}

	fake := NewFake()

	c.mu.Lock()
	defer c.mu.Unlock()

	c.resolver = fake

	return fake
}

func (c *Client) current() Resolver {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.resolver
}
