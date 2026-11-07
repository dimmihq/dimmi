/*
|--------------------------------------------------------------------------
| PII Redaction
|--------------------------------------------------------------------------
|
| Before customer text leaves for a model or lands in a trace, the bits
| that name a person or open an account are masked: emails, phone
| numbers, payment cards, IBANs, social security numbers and secrets
| such as API keys, tokens and passwords. Each mask is narrow on
| purpose: an order number, a date or an invoice id passes untouched,
| because the agent needs those to look things up and to answer.
|
*/

package pii

import (
	"regexp"
	"strings"
)

/*
Redactor holds the compiled masks, built once and safe for concurrent
use.
*/
type Redactor struct {
	secrets []*regexp.Regexp
	email   *regexp.Regexp
	iban    *regexp.Regexp
	card    *regexp.Regexp
	ssn     *regexp.Regexp
	phones  []*regexp.Regexp
}

/*
The placeholders masked values become.
*/
const (
	Email  = "[email]"
	Phone  = "[phone]"
	IBAN   = "[iban]"
	SSN    = "[ssn]"
	Secret = "[secret]"
)

/*
New compiles the masks.
*/
func New() *Redactor {
	return &Redactor{
		secrets: []*regexp.Regexp{
			regexp.MustCompile(`(?i)\b(bearer)\s+[A-Za-z0-9._~+/=-]{12,}`),
			regexp.MustCompile(`(?i)\b(api[_-]?key|access[_-]?token|token|secret|password|passwd|pwd)` +
				`(\s*[:=]\s*|\s+is\s+)\S{6,}`),
			regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}`),
			regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{16,}`),
			regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{20,}`),
			regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{20,}`),
			regexp.MustCompile(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`),
			regexp.MustCompile(`\bxox[abprs]-[A-Za-z0-9-]{10,}`),
		},
		email: regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`),
		iban:  regexp.MustCompile(`\b[A-Z]{2}\d{2}(?: ?[A-Z0-9]{4}){2,7}(?: ?[A-Z0-9]{1,3})?\b`),
		card:  regexp.MustCompile(`\b\d(?:[ -]?\d){12,18}\b`),
		ssn:   regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
		phones: []*regexp.Regexp{
			regexp.MustCompile(`\+\d{1,3}[\s.-]?\(?\d{1,4}\)?(?:[\s.-]?\d{2,4}){2,4}\b`),
			regexp.MustCompile(`(?:\(\d{3}\)\s?|\b\d{3}[.-])\d{3}[.-]\d{4}\b`),
			regexp.MustCompile(`\b0\d{2,4}[\s/]\d{3,4}[\s-]?\d{3,5}\b`),
		},
	}
}

/*
Redact masks every PII shape in the text. Secrets go first so a token
that happens to hold digits is never half-masked as a phone number.
*/
func (r *Redactor) Redact(text string) string {
	if text == "" {
		return text
	}

	out := text
	for _, p := range r.secrets {
		out = p.ReplaceAllStringFunc(out, maskSecret(p))
	}

	out = r.email.ReplaceAllString(out, Email)
	out = r.iban.ReplaceAllStringFunc(out, maskIBAN)
	out = r.card.ReplaceAllStringFunc(out, maskCard)
	out = r.ssn.ReplaceAllString(out, SSN)

	for _, p := range r.phones {
		out = p.ReplaceAllString(out, Phone)
	}

	return out
}

/*
RedactSecrets masks what must never be repeated even to its owner:
secrets, payment cards, IBANs and social security numbers. Emails and
phone numbers stay, since a lookup result naming the customer's own
address is what they asked for.
*/
func (r *Redactor) RedactSecrets(text string) string {
	if text == "" {
		return text
	}

	out := text
	for _, p := range r.secrets {
		out = p.ReplaceAllStringFunc(out, maskSecret(p))
	}

	out = r.iban.ReplaceAllStringFunc(out, maskIBAN)
	out = r.card.ReplaceAllStringFunc(out, maskCard)

	return r.ssn.ReplaceAllString(out, SSN)
}

/*
maskSecret keeps the label of a "password: x" or "Bearer x" pair, so
the text still reads, and replaces only the value.
*/
func maskSecret(p *regexp.Regexp) func(string) string {
	return func(m string) string {
		parts := p.FindStringSubmatch(m)
		switch len(parts) {
		case 2:
			return parts[1] + " " + Secret
		case 3:
			return parts[1] + parts[2] + Secret
		}
		return Secret
	}
}

/*
maskCard masks a digit run that passes the Luhn check as a payment
card, showing only its last four digits; any other run, such as an
order number, stays.
*/
func maskCard(run string) string {
	digits := onlyDigits(run)
	if len(digits) < 13 || len(digits) > 19 || !luhn(digits) {
		return run
	}
	return "[card ending " + digits[len(digits)-4:] + "]"
}

/*
maskIBAN masks an IBAN-shaped value whose mod-97 checksum holds.
*/
func maskIBAN(m string) string {
	compact := strings.ReplaceAll(m, " ", "")
	if len(compact) < 15 || len(compact) > 34 || !ibanValid(compact) {
		return m
	}
	return IBAN
}

/*
onlyDigits drops the separators of a digit run.
*/
func onlyDigits(s string) string {
	var b strings.Builder
	for _, c := range s {
		if c >= '0' && c <= '9' {
			b.WriteRune(c)
		}
	}
	return b.String()
}

/*
luhn reports whether the digits pass the Luhn checksum every payment
card number carries.
*/
func luhn(digits string) bool {
	sum := 0
	double := false
	for i := len(digits) - 1; i >= 0; i-- {
		d := int(digits[i] - '0')
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return sum%10 == 0
}

/*
ibanValid runs the ISO 13616 mod-97 check: the first four characters
move to the end, letters become numbers from 10, and the remainder
must be one.
*/
func ibanValid(iban string) bool {
	rotated := iban[4:] + iban[:4]
	rem := 0
	for _, c := range rotated {
		switch {
		case c >= '0' && c <= '9':
			rem = (rem*10 + int(c-'0')) % 97
		case c >= 'A' && c <= 'Z':
			rem = (rem*100 + int(c-'A'+10)) % 97
		default:
			return false
		}
	}
	return rem == 1
}
