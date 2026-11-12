/*
|--------------------------------------------------------------------------
| Durations and Variables
|--------------------------------------------------------------------------
|
| Durations read like Go's, "10m" or "2h", plus "d" for whole days of
| 24 hours, because people think about waits in days. Variables in text
| look like {{contact.first_name | fallback: "there"}}; only names the
| engine knows how to fill are accepted, so a typo is caught on publish
| instead of sending "Hi {{contact.frist_name}}" to a customer.
|
*/

package workflowspec

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
)

/*
ErrDuration is returned for a duration that cannot be read.
*/
var ErrDuration = errors.New("invalid duration")

/*
ParseDuration reads "30s", "10m", "2h" or "3d".
*/
func ParseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if days, ok := strings.CutSuffix(s, "d"); ok {
		n, err := strconv.Atoi(days)
		if err != nil || n < 0 {
			return 0, ErrDuration
		}

		return time.Duration(n) * 24 * time.Hour, nil
	}

	d, err := time.ParseDuration(s)
	if err != nil || d < 0 {
		return 0, ErrDuration
	}

	return d, nil
}

/*
variablePattern finds {{ name }} and {{ name | fallback: "x" }}.
*/
func variablePattern() *regexp.Regexp {
	return regexp.MustCompile(`\{\{\s*([a-z][a-z0-9_.]*)\s*(\|[^}]*)?\}\}`)
}

/*
Variables returns the variable names used in a text.
*/
func Variables(text string) []string {
	var out []string
	for _, m := range variablePattern().FindAllStringSubmatch(text, -1) {
		out = append(out, m[1])
	}

	return out
}

/*
KnownVariable reports whether the engine can fill the variable.
*/
func KnownVariable(name string) bool {
	switch name {
	case "contact.name", "contact.first_name", "contact.email", "contact.phone", "company.name",
		"conversation.number", "conversation.subject", "workspace.name", "schedule.next_open",
		"dimmi.summary", "dimmi.handover_reason", "ticket.reference", "ticket.title":
		return true
	}

	for _, prefix := range []string{"contact.custom.", "company.custom.", "run.vars.", "ticket.fields."} {
		if rest, ok := strings.CutPrefix(name, prefix); ok && rest != "" {
			return true
		}
	}

	return false
}
