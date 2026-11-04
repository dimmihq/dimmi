/*
|--------------------------------------------------------------------------
| System Resolver
|--------------------------------------------------------------------------
|
| Real lookups go through the standard library resolver. With a
| nameserver configured, every query goes to that server instead of the
| one the operating system knows.
|
*/

package dns

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"time"
)

/*
system resolves names with the standard library.
*/
type system struct {
	resolver *net.Resolver
	timeout  time.Duration
}

func newSystem(cfg Config) *system {
	resolver := &net.Resolver{}
	if cfg.Nameserver != "" {
		var dialer net.Dialer

		resolver.PreferGo = true
		resolver.Dial = func(ctx context.Context, network, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, cfg.Nameserver)
		}
	}

	return &system{resolver: resolver, timeout: cmp.Or(cfg.Timeout, DefaultTimeout)}
}

/*
TXT looks up the TXT records.
*/
func (s *system) TXT(ctx context.Context, name string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	records, err := s.resolver.LookupTXT(ctx, absolute(name))
	if err != nil {
		return nil, lookupError("txt", name, err)
	}

	return records, nil
}

/*
MX looks up the mail exchangers.
*/
func (s *system) MX(ctx context.Context, name string) ([]MX, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	records, err := s.resolver.LookupMX(ctx, absolute(name))
	if err != nil {
		return nil, lookupError("mx", name, err)
	}

	exchangers := make([]MX, len(records))
	for i, record := range records {
		exchangers[i] = MX{Host: relative(record.Host), Pref: record.Pref}
	}

	return exchangers, nil
}

/*
CNAME looks up the canonical name.
*/
func (s *system) CNAME(ctx context.Context, name string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	canonical, err := s.resolver.LookupCNAME(ctx, absolute(name))
	if err != nil {
		return "", lookupError("cname", name, err)
	}

	return relative(canonical), nil
}

/*
NS looks up the name servers.
*/
func (s *system) NS(ctx context.Context, name string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	records, err := s.resolver.LookupNS(ctx, absolute(name))
	if err != nil {
		return nil, lookupError("ns", name, err)
	}

	hosts := make([]string, len(records))
	for i, record := range records {
		hosts[i] = relative(record.Host)
	}

	return hosts, nil
}

/*
Addrs looks up the IPv4 and IPv6 addresses.
*/
func (s *system) Addrs(ctx context.Context, name string) ([]netip.Addr, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	addrs, err := s.resolver.LookupNetIP(ctx, "ip", absolute(name))
	if err != nil {
		return nil, lookupError("addrs", name, err)
	}

	for i, addr := range addrs {
		addrs[i] = addr.Unmap()
	}

	return addrs, nil
}

func lookupError(kind, name string, err error) error {
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
		return fmt.Errorf("lookup %s %s: %w", kind, name, ErrNotFound)
	}

	return fmt.Errorf("lookup %s %s: %w", kind, name, err)
}

func absolute(name string) string {
	return strings.TrimSuffix(name, ".") + "."
}

func relative(name string) string {
	return strings.TrimSuffix(name, ".")
}
