/*
|--------------------------------------------------------------------------
| PII Redaction
|--------------------------------------------------------------------------
|
| Before any text leaves for a model, the bits that name a person are
| masked: emails, phone numbers, card-looking numbers, bearer tokens.
| It is a cheap first line — the workspace's own redaction rules run
| at write time already — so a provider's log never holds what the
| customer wrote verbatim.
|
*/

package pii

import "regexp"

/*
Redactor holds the compiled masks, built once.
*/
type Redactor struct {
	patterns []*regexp.Regexp
}

/*
New compiles the masks.
*/
func New() *Redactor {
	return &Redactor{patterns: []*regexp.Regexp{
		regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`),
		regexp.MustCompile(`\b(?:\d[ -]?){13,19}\b`),
		regexp.MustCompile(`\+?\d[0-9 .()\-]{7,}\d`),
		regexp.MustCompile(`(?i)\b(bearer|api[_-]?key|token|secret|password)[:= ]+[A-Za-z0-9._\-]{8,}`),
		regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
	}}
}

/*
Redact masks PII-shaped text when the agent's setting asks for it.
*/
func (r *Redactor) Redact(text string, enabled bool) string {
	if !enabled || text == "" {
		return text
	}
	out := text
	for _, p := range r.patterns {
		out = p.ReplaceAllString(out, "[redacted]")
	}
	return out
}
