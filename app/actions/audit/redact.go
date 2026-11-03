/*
|--------------------------------------------------------------------------
| Redactor
|--------------------------------------------------------------------------
|
| An audit log is read by every admin of a workspace and may be shipped
| to other systems, so it never holds a secret, even one a feature put in
| its changes by mistake. Any value under a key that names a password,
| secret, token, key or code is replaced, however deep it sits.
|
*/

package audit

import (
	"regexp"
)

/*
Redacted replaces every secret value in an audit entry.
*/
const Redacted = "[redacted]"

/*
secretKey matches the keys whose values are secrets.
*/
var secretKey = regexp.MustCompile(`(?i)password|secret|token|key|code`)

/*
Redact returns a copy of the changes with the value of every secret
looking key, at any depth, replaced by Redacted.
*/
func Redact(changes map[string]any) map[string]any {
	if changes == nil {
		return map[string]any{}
	}

	out := make(map[string]any, len(changes))
	for k, v := range changes {
		if secretKey.MatchString(k) {
			out[k] = Redacted

			continue
		}

		out[k] = redactValue(v)
	}

	return out
}

/*
redactValue redacts the maps inside a value, lists of them included.
*/
func redactValue(v any) any {
	switch typed := v.(type) {
	case map[string]any:
		return Redact(typed)
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = redactValue(item)
		}

		return out
	default:
		return v
	}
}
