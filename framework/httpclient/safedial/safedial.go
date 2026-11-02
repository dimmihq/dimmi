/*
|--------------------------------------------------------------------------
| Safe Dialer
|--------------------------------------------------------------------------
|
| Users hand the application URLs and hosts to call: webhooks, crawls,
| model endpoints, mail servers. Any of them could point back inside the
| network, at the database, the cache or the cloud metadata service that
| hands out credentials. Every outbound connection is therefore checked
| against the address actually being connected to, after DNS resolution
| and on every redirect hop, so a name that resolves to a public address
| once and a private one later is still refused.
|
| Refused: loopback, private, link-local, carrier-grade NAT, multicast,
| IPv6 unique local, the metadata addresses and every other IANA special
| purpose range. Self-hosted installs reach intranet services in two ways:
| naming the exact host:port, which is preferred, or allowing private
| networks wholesale. Link-local addresses, where cloud metadata lives,
| stay refused even then.
|
*/

package safedial

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"syscall"
	"time"

	"code.dny.dev/ssrf"
)

/*
Errors from the dialer. ErrBlocked refuses a connection to an address
that is not allowed. ErrInvalidHost rejects an allowed host that is not
in host:port form.
*/
var (
	ErrBlocked     = errors.New("safedial: destination not allowed")
	ErrInvalidHost = errors.New("safedial: allowed host must be host:port")
)

/*
ConnectTimeout bounds establishing one connection.
*/
const ConnectTimeout = 5 * time.Second

/*
Options configure the dialer.

AllowPrivateNetworks permits loopback, private, carrier-grade NAT and
IPv6 unique local addresses. AllowHosts lists host:port pairs, such as
localhost:11434, that are dialed without any check.
*/
type Options struct {
	AllowPrivateNetworks bool
	AllowHosts           []string
}

/*
Dialer opens TCP connections, refusing addresses that are not allowed.
It is safe for concurrent use.
*/
type Dialer struct {
	guarded  *net.Dialer
	open     *net.Dialer
	guardian *ssrf.Guardian
	allowed  map[string]bool
}

/*
New returns a dialer for the options.
*/
func New(opts Options) (*Dialer, error) {
	allowed := make(map[string]bool, len(opts.AllowHosts))

	for _, host := range opts.AllowHosts {
		key, err := hostKey(host)
		if err != nil {
			return nil, fmt.Errorf("allow host %q: %w", host, ErrInvalidHost)
		}

		allowed[key] = true
	}

	d := &Dialer{
		open:     &net.Dialer{Timeout: ConnectTimeout},
		guardian: ssrf.New(guardianOptions(opts)...),
		allowed:  allowed,
	}
	d.guarded = &net.Dialer{Timeout: ConnectTimeout, Control: d.control}

	return d, nil
}

/*
DialContext connects to the address, a host:port. An allowed host is
dialed as is; any other is resolved and every address it resolves to is
checked right before connecting.
*/
func (d *Dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	dialer := d.guarded
	if key, err := hostKey(address); err == nil && d.allowed[key] {
		dialer = d.open
	}

	conn, err := dialer.DialContext(ctx, network, address)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", address, err)
	}

	return conn, nil
}

/*
Check reports, without connecting, whether the address would be
refused. Only IP addresses and localhost can be judged before resolving;
any other name passes here and is checked when dialed.
*/
func (d *Dialer) Check(address string) error {
	ip, port, ok := d.literal(address)
	if !ok {
		return nil
	}

	network := "tcp6"
	if ip.Is4() {
		network = "tcp4"
	}

	return d.control(network, net.JoinHostPort(ip.String(), port), nil)
}

/*
literal returns the IP and port of an address whose destination is
known without resolving, unless the address is allowed by name.
*/
func (d *Dialer) literal(address string) (netip.Addr, string, bool) {
	key, err := hostKey(address)
	if err != nil || d.allowed[key] {
		return netip.Addr{}, "", false
	}

	host, port, _ := net.SplitHostPort(key)
	if host == "localhost" {
		host = "127.0.0.1"
	}

	ip, err := netip.ParseAddr(host)

	return ip.Unmap(), port, err == nil
}

/*
control runs right before each connection attempt, on the resolved
address.
*/
func (d *Dialer) control(network, address string, conn syscall.RawConn) error {
	if addrPort, err := netip.ParseAddrPort(address); err == nil && addrPort.Addr().String() == awsMetadataV6 {
		return fmt.Errorf("%w: %s is a cloud metadata address", ErrBlocked, address)
	}

	if err := d.guardian.Safe(network, address, conn); err != nil {
		return fmt.Errorf("%w: %w", ErrBlocked, err)
	}

	return nil
}

/*
awsMetadataV6 is the IPv6 metadata address on EC2. It sits in the unique
local range that allowing private networks opens up, so it is refused on
its own.
*/
const awsMetadataV6 = "fd00:ec2::254"

/*
guardianOptions lets any port through and, when private networks are
allowed, opens their ranges while link-local stays refused.
*/
func guardianOptions(o Options) []ssrf.Option {
	opts := []ssrf.Option{ssrf.WithAnyPort()}
	if !o.AllowPrivateNetworks {
		return opts
	}

	return append(opts,
		ssrf.WithAllowedV4Prefixes(
			netip.MustParsePrefix("127.0.0.0/8"),
			netip.MustParsePrefix("10.0.0.0/8"),
			netip.MustParsePrefix("172.16.0.0/12"),
			netip.MustParsePrefix("192.168.0.0/16"),
			netip.MustParsePrefix("100.64.0.0/10"),
		),
		ssrf.WithAllowedV6Prefixes(
			netip.MustParsePrefix("::1/128"),
			netip.MustParsePrefix("fc00::/7"),
		),
	)
}

/*
hostKey normalizes a host:port so the configured form and the dialed
form compare equal.
*/
func hostKey(address string) (string, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil || host == "" || port == "" {
		return "", ErrInvalidHost
	}

	return net.JoinHostPort(strings.ToLower(host), port), nil
}
