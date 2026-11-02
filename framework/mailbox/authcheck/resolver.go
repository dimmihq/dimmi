/*
|--------------------------------------------------------------------------
| DNS For The Checks
|--------------------------------------------------------------------------
|
| The SPF, DKIM and DMARC libraries look records up through the
| application's resolver, so lookups share its timeout and tests answer
| from a fake zone. The libraries tell a missing record from a failed
| lookup by the standard library's DNS error, so answers are translated
| into it: a missing record is final, anything else may pass on retry.
|
*/

package authcheck

import (
	"context"
	"errors"
	"net"

	"github.com/dimmihq/dimmi/framework/dns"
)

/*
lookupTXT returns a TXT lookup bound to the context, as the DKIM and
DMARC libraries take one.
*/
func lookupTXT(ctx context.Context, resolver dns.Resolver) func(name string) ([]string, error) {
	return func(name string) ([]string, error) {
		records, err := resolver.TXT(ctx, name)
		if err != nil {
			return nil, dnsError(name, err)
		}

		return records, nil
	}
}

/*
spfResolver answers the SPF library's lookups from the resolver. Reverse
lookups are not supported, so the rarely used ptr mechanism never
matches.
*/
type spfResolver struct {
	dns dns.Resolver
}

/*
LookupTXT returns the TXT records of the name.
*/
func (r spfResolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	return lookupTXT(ctx, r.dns)(name)
}

/*
LookupMX returns the mail exchangers of the name.
*/
func (r spfResolver) LookupMX(ctx context.Context, name string) ([]*net.MX, error) {
	exchangers, err := r.dns.MX(ctx, name)
	if err != nil {
		return nil, dnsError(name, err)
	}

	out := make([]*net.MX, 0, len(exchangers))
	for _, mx := range exchangers {
		out = append(out, &net.MX{Host: mx.Host, Pref: mx.Pref})
	}

	return out, nil
}

/*
LookupIPAddr returns the addresses of the host.
*/
func (r spfResolver) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	addrs, err := r.dns.Addrs(ctx, host)
	if err != nil {
		return nil, dnsError(host, err)
	}

	out := make([]net.IPAddr, 0, len(addrs))
	for _, addr := range addrs {
		out = append(out, net.IPAddr{IP: net.IP(addr.AsSlice())})
	}

	return out, nil
}

/*
LookupAddr finds no names: reverse lookups are not supported.
*/
func (r spfResolver) LookupAddr(_ context.Context, addr string) ([]string, error) {
	return nil, dnsError(addr, dns.ErrNotFound)
}

func dnsError(name string, err error) error {
	if errors.Is(err, dns.ErrNotFound) {
		return &net.DNSError{Err: "no such host", Name: name, IsNotFound: true}
	}

	return &net.DNSError{Err: err.Error(), Name: name, IsTemporary: true}
}
