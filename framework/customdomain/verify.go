/*
|--------------------------------------------------------------------------
| Domain Verification
|--------------------------------------------------------------------------
|
| Before serving a customer's pages on a hostname, two things must hold.
| Ownership: the customer controls the hostname's DNS, shown by a TXT
| record holding a value only they were given. Without it, anyone could
| claim a hostname that still points here from an old setup. Routing:
| requests for the hostname reach this application, through a CNAME to
| the configured target, addresses among the configured ones, or failing
| both a probe that gets this application's own answer.
|
| A record that is missing or wrong is a finding the customer can fix,
| not an error. Only a lookup that could not be answered at all, such as
| a timeout, is returned as an error, so the caller tries again later.
|
*/

package customdomain

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/dns"
	"github.com/dimmihq/dimmi/framework/httpclient"
)

/*
The findings of a check, each naming what the customer has to fix.

ErrOwnershipMissing means no TXT record exists at the ownership name.
ErrOwnershipMismatch means TXT records exist but none holds the value.
ErrRoutingMismatch means the hostname neither points at the target nor
answers the probe.
*/
var (
	ErrOwnershipMissing  = errors.New("customdomain: ownership record missing")
	ErrOwnershipMismatch = errors.New("customdomain: ownership record does not hold the expected value")
	ErrRoutingMismatch   = errors.New("customdomain: hostname does not route here")
)

/*
Expect describes the records a hostname must have.

TXTName and TXTValue are the ownership record; an empty TXTName skips
the ownership check. CNAME is the routing target for a subdomain, and
Addrs the addresses an apex may point at. ProbeToken, when set, runs the
probe if the records do not show routing.
*/
type Expect struct {
	TXTName    string
	TXTValue   string
	CNAME      string
	Addrs      []netip.Addr
	ProbeToken string
}

/*
Result is what a check found. Ownership and Routing report the two
conditions, Probed whether the probe answered correctly, and Findings
explain each condition that does not hold.
*/
type Result struct {
	Ownership bool
	Routing   bool
	Probed    bool
	Findings  []error
}

/*
Verified reports whether the hostname is owned and routed here.
*/
func (r Result) Verified() bool {
	return r.Ownership && r.Routing
}

/*
Verifier checks hostnames through DNS and the routing probe.
*/
type Verifier struct {
	resolver dns.Resolver
	client   *httpclient.Client
	probe    Probe
}

/*
NewVerifier returns a verifier that looks records up with the resolver
and runs the probe through the HTTP client.
*/
func NewVerifier(resolver dns.Resolver, client *httpclient.Client, probe Probe) *Verifier {
	return &Verifier{resolver: resolver, client: client, probe: probe}
}

/*
Check looks at the hostname's records and reports what holds. It
returns an error only when a lookup fails for a reason other than the
record not existing.
*/
func (v *Verifier) Check(ctx context.Context, host string, expect Expect) (Result, error) {
	var result Result

	finding, err := v.ownership(ctx, expect)
	if err != nil {
		return Result{}, err
	}

	result.Ownership = finding == nil
	if finding != nil {
		result.Findings = append(result.Findings, finding)
	}

	routed, err := v.records(ctx, host, expect)
	if err != nil {
		return Result{}, err
	}

	if !routed && expect.ProbeToken != "" {
		result.Probed = v.probe.Run(ctx, v.client, host, expect.ProbeToken)
		routed = result.Probed
	}

	result.Routing = routed
	if !routed {
		result.Findings = append(result.Findings, fmt.Errorf("%w: %s", ErrRoutingMismatch, host))
	}

	return result, nil
}

/*
ownership returns a finding when the ownership record is missing or
wrong, and an error when it could not be looked up.
*/
func (v *Verifier) ownership(ctx context.Context, expect Expect) (finding, err error) {
	if expect.TXTName == "" {
		return nil, nil
	}

	values, err := v.resolver.TXT(ctx, expect.TXTName)
	if errors.Is(err, dns.ErrNotFound) {
		return fmt.Errorf("%w: %s", ErrOwnershipMissing, expect.TXTName), nil
	}

	if err != nil {
		return nil, fmt.Errorf("check ownership of %s: %w", expect.TXTName, err)
	}

	if !slices.Contains(values, expect.TXTValue) {
		return fmt.Errorf("%w: %s", ErrOwnershipMismatch, expect.TXTName), nil
	}

	return nil, nil
}

/*
records reports whether the hostname's CNAME is the target, or failing
that whether every address it resolves to is an expected one.
*/
func (v *Verifier) records(ctx context.Context, host string, expect Expect) (bool, error) {
	if expect.CNAME != "" {
		canonical, err := v.resolver.CNAME(ctx, host)
		if err != nil && !errors.Is(err, dns.ErrNotFound) {
			return false, fmt.Errorf("check routing of %s: %w", host, err)
		}

		if err == nil && sameName(canonical, expect.CNAME) {
			return true, nil
		}
	}

	if len(expect.Addrs) == 0 {
		return false, nil
	}

	addrs, err := v.resolver.Addrs(ctx, host)
	if errors.Is(err, dns.ErrNotFound) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("check routing of %s: %w", host, err)
	}

	foreign := func(a netip.Addr) bool { return !slices.Contains(expect.Addrs, a.Unmap()) }

	return !slices.ContainsFunc(addrs, foreign), nil
}

func sameName(a, b string) bool {
	return strings.EqualFold(strings.TrimSuffix(a, "."), strings.TrimSuffix(b, "."))
}
