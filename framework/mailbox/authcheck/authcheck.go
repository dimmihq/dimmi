/*
|--------------------------------------------------------------------------
| Sender Authentication
|--------------------------------------------------------------------------
|
| Anyone can write any From address. What can be checked is whether the
| sending server was allowed to send for a domain (SPF), whether a domain
| signed the message (DKIM), and whether either of those domains matches
| the From domain under the policy that domain publishes (DMARC).
|
| This file reports those facts as standard results and leaves deciding
| what they mean to the caller. SPF needs the connecting server's
| address, known only to whoever received the connection: without it,
| a verdict the receiving provider already reached is taken instead.
|
| DNS failures never block the check: they come back as temporary error
| results, and the caller decides whether to wait or move on.
|
*/

package authcheck

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/mail"
	"net/netip"
	"strings"

	"blitiri.com.ar/go/spf"
	"github.com/emersion/go-msgauth/dkim"

	"github.com/dimmihq/dimmi/framework/dns"
)

/*
Result is one check's outcome, named as in Authentication-Results
headers.
*/
type Result string

/*
Results a check can have.
*/
const (
	Pass      Result = "pass"
	Fail      Result = "fail"
	SoftFail  Result = "softfail"
	Neutral   Result = "neutral"
	None      Result = "none"
	TempError Result = "temperror"
	PermError Result = "permerror"
)

/*
maxSignatures bounds how many DKIM signatures one message may make us
verify, each costing a DNS lookup.
*/
const maxSignatures = 5

/*
Input is what is known about one received message.

RemoteIP is the connecting server and HELO the name it gave; MailFrom is
the envelope sender. SPF is a verdict the receiving provider already
reached, used when RemoteIP is not valid.
*/
type Input struct {
	Raw      []byte
	RemoteIP netip.Addr
	HELO     string
	MailFrom string
	SPF      Result
}

/*
DKIMResult is the outcome of one DKIM signature, by signing domain.
*/
type DKIMResult struct {
	Domain string
	Result Result
}

/*
Results are the outcomes for one message.

SPFDomain is the domain SPF speaks for: the envelope sender's, or the
HELO name for the null sender. DMARCPolicy is the policy the From domain
publishes, empty without a record. AlignedDomain is the domain of a
passing SPF or DKIM result that matches the From domain under the
record's alignment modes, relaxed when there is no record; it is set
even when the domain publishes no DMARC record.
*/
type Results struct {
	FromDomain    string
	SPF           Result
	SPFDomain     string
	DKIM          []DKIMResult
	DMARC         Result
	DMARCPolicy   string
	AlignedDomain string
}

/*
Checker runs the checks, looking records up through the resolver. It is
safe for concurrent use.
*/
type Checker struct {
	resolver dns.Resolver
}

/*
New returns a checker using the resolver.
*/
func New(resolver dns.Resolver) *Checker {
	return &Checker{resolver: resolver}
}

/*
Verify checks the message.
*/
func (c *Checker) Verify(ctx context.Context, in Input) Results {
	results := Results{FromDomain: fromDomain(in.Raw)}
	results.SPF, results.SPFDomain = c.spf(ctx, in)
	results.DKIM = c.dkim(ctx, in.Raw)
	c.dmarc(ctx, &results)

	return results
}

func (c *Checker) spf(ctx context.Context, in Input) (Result, string) {
	domain := domainOf(in.MailFrom)
	if domain == "" {
		domain = strings.ToLower(in.HELO)
	}

	if !in.RemoteIP.IsValid() {
		if in.SPF == "" {
			return None, domain
		}

		return in.SPF, domain
	}

	result, _ := spf.CheckHostWithSender(
		net.IP(in.RemoteIP.AsSlice()), in.HELO, in.MailFrom,
		spf.WithContext(ctx), spf.WithResolver(spfResolver{dns: c.resolver}),
	)

	return Result(result), domain
}

func (c *Checker) dkim(ctx context.Context, raw []byte) []DKIMResult {
	verifications, err := dkim.VerifyWithOptions(bytes.NewReader(raw), &dkim.VerifyOptions{
		LookupTXT:        lookupTXT(ctx, c.resolver),
		MaxVerifications: maxSignatures,
	})
	if err != nil && !errors.Is(err, dkim.ErrTooManySignatures) {
		return nil
	}

	results := make([]DKIMResult, 0, len(verifications))
	for _, v := range verifications {
		results = append(results, DKIMResult{Domain: strings.ToLower(v.Domain), Result: dkimResult(v.Err)})
	}

	return results
}

func dkimResult(err error) Result {
	switch {
	case err == nil:
		return Pass
	case dkim.IsTempFail(err):
		return TempError
	case dkim.IsPermFail(err):
		return PermError
	default:
		return Fail
	}
}

/*
fromDomain returns the lowercased domain of the first From address, or
empty when the message has none that can be read.
*/
func fromDomain(raw []byte) string {
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return ""
	}

	from, err := msg.Header.AddressList("From")
	if err != nil || len(from) == 0 {
		return ""
	}

	return domainOf(from[0].Address)
}

func domainOf(address string) string {
	at := strings.LastIndexByte(address, '@')
	if at < 0 {
		return ""
	}

	return strings.ToLower(strings.TrimSuffix(address[at+1:], "."))
}
