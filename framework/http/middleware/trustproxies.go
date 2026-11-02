/*
|--------------------------------------------------------------------------
| Trusted Proxies
|--------------------------------------------------------------------------
|
| Behind a load balancer every connection comes from the balancer, and the
| client's address travels in a forwarding header that anyone can forge.
| The header is read only when the connection comes from a trusted proxy,
| and then from the right, skipping every trusted hop, so the address
| used for rate limits and audit logs is the last one nobody could fake.
|
*/

package middleware

import (
	"fmt"
	"net"
	"net/http"

	"github.com/realclientip/realclientip-go"

	fhttp "github.com/dimmihq/dimmi/framework/http"
)

/*
DefaultForwardedHeader is the header TrustProxies reads when given none.
*/
const DefaultForwardedHeader = "X-Forwarded-For"

/*
TrustProxies works out the client's address from the header when the
connection comes from one of the trusted addresses or ranges, such as
"10.0.0.0/8". Handlers read it with http.ClientIP.
*/
func TrustProxies(trusted []string, header string) (fhttp.Middleware, error) {
	if header == "" {
		header = DefaultForwardedHeader
	}

	ranges, err := realclientip.AddressesAndRangesToIPNets(trusted...)
	if err != nil {
		return nil, fmt.Errorf("parse trusted proxies: %w", err)
	}

	strategy, err := realclientip.NewRightmostTrustedRangeStrategy(header, ranges)
	if err != nil {
		return nil, fmt.Errorf("trusted proxies: %w", err)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !fromTrusted(r.RemoteAddr, ranges) {
				next.ServeHTTP(w, r)

				return
			}

			if ip := strategy.ClientIP(r.Header, r.RemoteAddr); ip != "" {
				r = r.WithContext(fhttp.WithClientIP(r.Context(), ip))
			}

			next.ServeHTTP(w, r)
		})
	}, nil
}

func fromTrusted(remoteAddr string, ranges []net.IPNet) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}

	for _, r := range ranges {
		if r.Contains(ip) {
			return true
		}
	}

	return false
}
