/*
|--------------------------------------------------------------------------
| Forwarding Test Findings
|--------------------------------------------------------------------------
|
| A test that fails should say why in words the person setting up mail
| can act on. A slow or rewritten forward still works but is worth
| knowing; a bounce names the mail service's refusal; a test that never
| came back lists what usually holds a forward up in the mail service the
| address seems to use. Spam at the host is always one of them.
|
*/

package email

import (
	"strings"
	"time"

	inbound "github.com/dimmihq/dimmi/app/integrations/email"
	"github.com/dimmihq/dimmi/app/integrations/email/setupguides"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The severities of a finding.
*/
const (
	findingError = "error"
	findingWarn  = "warn"
	findingInfo  = "info"
)

/*
externalForwardBlocked is the status Microsoft 365 answers with when
its outbound policy refuses to forward mail outside the organization.
*/
const externalForwardBlocked = "5.7.520"

/*
receivedFindings returns what a test that came back learned: a forward
slower than five minutes, and a subject the forward changed.
*/
func receivedFindings(latency time.Duration, sentSubject, receivedSubject string) []TestFinding {
	findings := []TestFinding{}

	if latency > testSlowAfter {
		findings = append(findings, TestFinding{Code: "slow_forward", Severity: findingInfo})
	}

	if inbound.NormalizeSubject(receivedSubject) != inbound.NormalizeSubject(sentSubject) {
		findings = append(findings, TestFinding{Code: "subject_rewritten", Severity: findingInfo})
	}

	return findings
}

/*
bounceFindings returns what a bounced test mail says: Microsoft 365
refusing to forward outside the organization, or a plain bounce.
*/
func bounceFindings(status, diagnostic string) []TestFinding {
	if strings.Contains(status+" "+diagnostic, externalForwardBlocked) {
		return []TestFinding{{Code: "m365_external_forward_blocked", Severity: findingError}}
	}

	return []TestFinding{{Code: "test_bounced", Severity: findingError}}
}

/*
timeoutHints returns what usually holds a forward up in the mail service
of the guide, a Gmail confirmation the address still waits on first,
spam at the host last.
*/
func timeoutHints(guide string, address models.EmailAddress) []TestFinding {
	findings := []TestFinding{}

	if address.ForwardingConfirmation != nil {
		findings = append(findings, TestFinding{Code: "gmail_confirmation_pending", Severity: findingError})
	}

	switch guide {
	case setupguides.Gmail:
		findings = append(findings, TestFinding{Code: "gmail_not_confirmed", Severity: findingWarn})
	case setupguides.GoogleWorkspace:
		findings = append(findings, TestFinding{Code: "google_rule_delay", Severity: findingInfo})
	case setupguides.GoogleGroup:
		findings = append(findings, TestFinding{Code: "group_moderation", Severity: findingWarn},
			TestFinding{Code: "google_rule_delay", Severity: findingInfo})
	case setupguides.Microsoft365:
		findings = append(findings, TestFinding{Code: "m365_forward_policy", Severity: findingWarn})
	}

	return append(findings, TestFinding{Code: "check_spam", Severity: findingInfo})
}
