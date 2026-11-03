/*
|--------------------------------------------------------------------------
| Forwarding Confirmations
|--------------------------------------------------------------------------
|
| Gmail, and Zoho in its own way, will not forward to a new address
| until its owner confirms: they mail the forwarding address a code and
| a link. That mail arrives here, so its code and link are read out and
| shown to the workspace's admins, who finish the setup themselves. The
| link is never followed by the install: confirming forwarding is the
| mailbox owner's decision.
|
*/

package email

import (
	"regexp"
	"strings"
)

/*
Mail providers that send forwarding confirmations.
*/
const (
	ProviderGmail = "gmail"
	ProviderZoho  = "zoho"
)

var (
	gmailCode       = regexp.MustCompile(`\(#(\d+)\)`)
	gmailConfirmURL = regexp.MustCompile(`https://(mail-settings\.google\.com|mail\.google\.com)/mail/[^\s<>"')\]]+`)
	zohoConfirmURL  = regexp.MustCompile(`https://[a-z0-9.-]*zoho\.[a-z.]+/[^\s<>"')\]]+`)
	numericCode     = regexp.MustCompile(`\b(\d{6,10})\b`)
)

/*
Confirmation is what a forwarding confirmation mail asks for: a code
to type, a link to open, or both.
*/
type Confirmation struct {
	Provider   string `json:"provider"`
	Code       string `json:"code"`
	ConfirmURL string `json:"confirm_url"`
	From       string `json:"from"`
}

/*
FindConfirmation reads a forwarding confirmation mail from its sender,
subject and body, and reports whether it is one.
*/
func FindConfirmation(from, subject, body string) (Confirmation, bool) {
	from = strings.ToLower(from)

	switch {
	case from == "forwarding-noreply@google.com" && gmailCode.MatchString(subject):
		return Confirmation{
			Provider: ProviderGmail, Code: gmailCode.FindStringSubmatch(subject)[1],
			ConfirmURL: gmailConfirmURL.FindString(body), From: from,
		}, true
	case strings.HasSuffix(from, "@zohoaccounts.com") && strings.Contains(strings.ToLower(subject), "forward"):
		code := ""
		if match := numericCode.FindStringSubmatch(body); match != nil {
			code = match[1]
		}

		return Confirmation{
			Provider: ProviderZoho, Code: code, ConfirmURL: zohoConfirmURL.FindString(body), From: from,
		}, true
	}

	return Confirmation{}, false
}
