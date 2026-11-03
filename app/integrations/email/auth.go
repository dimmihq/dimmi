/*
|--------------------------------------------------------------------------
| Sender Authentication
|--------------------------------------------------------------------------
|
| Anyone can write any From address. A sender is believed only when
| DMARC passes: a DKIM signature, or an SPF verdict, whose domain aligns
| with the From domain. Only then does the address count as proven,
| which lets it join the history of a verified customer.
|
| The install never sees the connecting server, so SPF comes only from
| the receiving provider's verdict, and Authentication-Results headers
| are never trusted: anyone upstream could have written them. DKIM is
| verified here, since signatures survive forwarding. A DNS failure
| leaves the result unknown and never holds up the mail.
|
*/

package email

import (
	"slices"

	"github.com/dimmihq/dimmi/framework/mailbox"
	"github.com/dimmihq/dimmi/framework/mailbox/authcheck"
)

/*
Authentication results.
*/
const (
	AuthPass    = "pass"
	AuthFail    = "fail"
	AuthNone    = "none"
	AuthUnknown = "unknown"
)

/*
Auth is what is known about a sender's authenticity. Result is pass
when DMARC aligned; AlignedDomain is the domain that aligned.
*/
type Auth struct {
	SPF           string `json:"spf"`
	DKIM          string `json:"dkim"`
	DMARC         string `json:"dmarc"`
	Result        string `json:"result"`
	AlignedDomain string `json:"aligned_domain,omitempty"`
	DMARCPolicy   string `json:"dmarc_policy,omitempty"`
	Source        string `json:"source"`
}

/*
Passed reports whether the sender is proven.
*/
func (a Auth) Passed() bool {
	return a.Result == AuthPass
}

/*
SPFVerdict maps a provider's SPF verdict to the result DMARC alignment
reads, none when the provider gave none.
*/
func SPFVerdict(v mailbox.Verdict) authcheck.Result {
	switch v {
	case mailbox.VerdictPass:
		return authcheck.Pass
	case mailbox.VerdictFail:
		return authcheck.Fail
	case mailbox.VerdictUnknown:
		return authcheck.TempError
	case mailbox.VerdictNone:
		return authcheck.None
	}

	return authcheck.None
}

/*
Evaluate decides the authentication result from the checks run here
and the provider's verdicts.
*/
func Evaluate(results authcheck.Results, verdicts mailbox.Verdicts, source string) Auth {
	auth := Auth{
		SPF: string(results.SPF), DKIM: dkimSummary(results.DKIM), DMARC: string(results.DMARC),
		DMARCPolicy: results.DMARCPolicy, Source: source,
	}

	switch {
	case verdicts.DMARC == mailbox.VerdictFail:
		auth.Result = AuthFail
	case results.AlignedDomain != "":
		auth.Result, auth.AlignedDomain = AuthPass, results.AlignedDomain
	case results.DMARC == authcheck.TempError || verdicts.DMARC == mailbox.VerdictUnknown || auth.DKIM == AuthUnknown:
		auth.Result = AuthUnknown
	case results.DMARC == authcheck.Fail && (results.DMARCPolicy == "quarantine" || results.DMARCPolicy == "reject"):
		auth.Result = AuthFail
	default:
		auth.Result = AuthNone
	}

	return auth
}

/*
dkimSummary sums up the signatures: pass when any passed, unknown when
a lookup failed, fail when any failed, none without signatures.
*/
func dkimSummary(results []authcheck.DKIMResult) string {
	has := func(want authcheck.Result) bool {
		return slices.ContainsFunc(results, func(r authcheck.DKIMResult) bool { return r.Result == want })
	}

	switch {
	case has(authcheck.Pass):
		return AuthPass
	case has(authcheck.TempError):
		return AuthUnknown
	case len(results) > 0:
		return AuthFail
	}

	return AuthNone
}
