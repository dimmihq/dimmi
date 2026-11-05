/*
|--------------------------------------------------------------------------
| Signup Risk
|--------------------------------------------------------------------------
|
| A public server sees scripted signups. Two signs give them away: an
| address at a throwaway mailbox, and many accounts from one network in
| a day. A throwaway address is refused or marked, as the server is
| configured; a busy network only marks the account, since a school or
| an office shares one. Marks are for people reviewing accounts later and
| change nothing at signup. Someone invited by a member is never judged:
| the member vouched for the address.
|
*/

package auth

import (
	"context"
	"net/netip"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/ratelimit"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/config"
)

/*
The marks a signup can earn, and how many signups one network makes in a
day before they are marked.
*/
const (
	RiskDisposableEmail = "disposable_email"
	RiskSignupVelocity  = "signup_velocity"
	signupsPerNetwork   = 20
)

/*
SignupRisk judges a signup. Disposable holds the throwaway domains, Mode
is the configured treatment, and Limiter counts signups per network.
*/
type SignupRisk struct {
	Disposable map[string]struct{}
	Mode       string
	Limiter    *ratelimit.Limiter
}

/*
Assess returns the marks a signup from the email at the address earns.

Errors: [exceptions.ErrDisposableEmail] for a throwaway address on a
server that refuses them.
*/
func (r *SignupRisk) Assess(ctx context.Context, email, ip string) ([]string, error) {
	var marks []string

	if r.Mode != config.DisposableOff && r.disposable(email) {
		if r.Mode == config.DisposableBlock {
			return nil, exceptions.ErrDisposableEmail
		}

		marks = append(marks, RiskDisposableEmail)
	}

	if network := networkOf(ip); network != "" {
		res, err := r.Limiter.Attempt(ctx, "signup:network:"+network, signupsPerNetwork, 24*time.Hour)
		if err == nil && !res.Allowed {
			marks = append(marks, RiskSignupVelocity)
		}
	}

	return marks, nil
}

/*
disposable reports whether the email's domain, or a domain it sits
under, is a throwaway mailbox.
*/
func (r *SignupRisk) disposable(email string) bool {
	at := strings.LastIndex(email, "@")
	if at < 0 {
		return false
	}

	domain := strings.ToLower(email[at+1:])
	for domain != "" {
		if _, ok := r.Disposable[domain]; ok {
			return true
		}

		_, domain, _ = strings.Cut(domain, ".")
	}

	return false
}

/*
networkOf returns the network an address belongs to: its /24 for IPv4,
its /48 for IPv6, or "" for no address.
*/
func networkOf(ip string) string {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return ""
	}

	bits := 48
	if addr.Is4() || addr.Is4In6() {
		addr, bits = addr.Unmap(), 24
	}

	prefix, err := addr.Prefix(bits)
	if err != nil {
		return ""
	}

	return prefix.String()
}
