/*
|--------------------------------------------------------------------------
| Form Relays
|--------------------------------------------------------------------------
|
| Contact forms on Webflow, Typeform and the like mail their entries
| from their own no-reply address, with the visitor in Reply-To. The
| customer is the visitor, not the form service, so the Reply-To
| address becomes the contact. Any no-reply sender with exactly one
| Reply-To on another domain is treated the same way. A form that sets
| no Reply-To often prints the visitor's address in the body instead.
|
*/

package email

import (
	"regexp"
	"strings"
)

var bodyEmailField = regexp.MustCompile(`(?im)^\s*\|?\s*\**e-?mail(\s+address)?\**\s*(:\**\s*\|?|\|)\s*\**` +
	`\[?([^\s<>\]()|]+@[^\s<>\]()|]+\.[a-z]{2,})`)

/*
FormRelay returns the form service that sent the message on a
visitor's behalf, by its sender address and Reply-To addresses.
*/
func FormRelay(from string, replyTo []string) (string, bool) {
	from = strings.ToLower(from)
	_, domain, _ := strings.Cut(from, "@")

	for _, service := range formServices() {
		if domain == service || strings.HasSuffix(domain, "."+service) {
			return service, true
		}
	}

	if from == "forms-receipts-noreply@google.com" {
		return "google.com", true
	}

	if !Noreply(from) || len(replyTo) != 1 {
		return "", false
	}

	_, replyDomain, _ := strings.Cut(strings.ToLower(replyTo[0]), "@")
	if replyDomain == "" || replyDomain == domain {
		return "", false
	}

	return domain, true
}

/*
BodyEmail returns the address a form printed in its body as an Email
field, on a line of its own or in a table row, or "".
*/
func BodyEmail(text string) string {
	match := bodyEmailField.FindStringSubmatch(text)
	if match == nil {
		return ""
	}

	return strings.ToLower(strings.ReplaceAll(match[3], `\`, ""))
}

/*
formServices lists the domains form services send entries from.
*/
func formServices() []string {
	return []string{
		"webflow.com", "typeform.com", "tally.so", "framer.com", "framerusercontent.com", "formspree.io",
		"jotform.com", "netlify.com", "wix-forms.com", "squarespace.com", "hubspot.com", "usebasin.com",
		"getform.io", "formsubmit.co",
	}
}
