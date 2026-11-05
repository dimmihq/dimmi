/*
|--------------------------------------------------------------------------
| Receiving Host's Verdicts
|--------------------------------------------------------------------------
|
| A host that received mail writes its findings in an
| Authentication-Results header on top of the message. Anyone can write
| that header further down, so only the topmost is read, and only when it
| names the host expected.
|
*/

package webhook

import (
	"strings"

	"github.com/emersion/go-msgauth/authres"

	"github.com/dimmihq/dimmi/framework/mailbox"
)

/*
TopAuthResults reads the first Authentication-Results header of a raw
message. With a server given it must name that server. It reports
whether the header could be believed.
*/
func TopAuthResults(raw []byte, server string) (mailbox.Verdicts, bool) {
	top := First(Headers(raw), "Authentication-Results")
	if top == "" {
		return mailbox.Verdicts{}, false
	}

	identifier, results, err := authres.Parse(top)
	if err != nil || identifier == "" || (server != "" && !strings.EqualFold(identifier, server)) {
		return mailbox.Verdicts{}, false
	}

	var v mailbox.Verdicts

	for _, result := range results {
		switch r := result.(type) {
		case *authres.SPFResult:
			v.SPF = fromResult(r.Value)
		case *authres.DKIMResult:
			if v.DKIM != mailbox.VerdictPass {
				v.DKIM = fromResult(r.Value)
			}
		case *authres.DMARCResult:
			v.DMARC = fromResult(r.Value)
		}
	}

	return v, true
}

func fromResult(value authres.ResultValue) mailbox.Verdict {
	return Verdict(string(value))
}
