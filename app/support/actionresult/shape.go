/*
|--------------------------------------------------------------------------
| Result Shaping
|--------------------------------------------------------------------------
|
| Only what the admin mapped reaches the model: each response field is a
| plain dotted path into the body, and a path that finds nothing is left
| out rather than sent as null. Lists are cut to twenty items and say
| so, secrets and card numbers are masked, and the whole result stays
| under a few kilobytes, so one chatty API can never flood a prompt or
| leak what the customer should not see.
|
*/

package actionresult

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/tidwall/gjson"

	"github.com/dimmihq/dimmi/app/support/pii"
)

/*
Shaping limits: items kept of a list, the length of a path, the length
of an upstream error message and the text kept of a plain-text result.
*/
const (
	maxArray      = 20
	smallArray    = 5
	maxPath       = 200
	maxErrorChars = 300
	maxTextChars  = 4000
	maxFields     = 30
)

/*
DefaultMaxBytes caps a shaped result.
*/
const DefaultMaxBytes = 8 << 10

/*
ErrInvalidResponse marks a body that is not the JSON the action expects.
ErrInvalidPath marks a response path outside the plain dotted form.
*/
var (
	ErrInvalidResponse = errors.New("invalid response")
	ErrInvalidPath     = errors.New("invalid response path")
)

/*
ResponseField maps one value of the response to a name the model reads.
*/
type ResponseField struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Description string `json:"description,omitempty"`
}

/*
pathPattern matches the plain dotted paths the subset allows: keys,
indexes and the # that maps over a list.
*/
func pathPattern() *regexp.Regexp {
	return regexp.MustCompile(`^[A-Za-z0-9_#-]+(\.[A-Za-z0-9_#-]+)*$`)
}

/*
namePattern matches a field name.
*/
func namePattern() *regexp.Regexp {
	return regexp.MustCompile(`^[a-z][a-z0-9_]{0,47}$`)
}

/*
CheckFields validates the mapped fields: plain paths without modifiers,
pipes or multipaths, and unique names.
*/
func CheckFields(fields []ResponseField) error {
	if len(fields) > maxFields {
		return fmt.Errorf("%w: at most %d fields", ErrInvalidPath, maxFields)
	}
	seen := map[string]bool{}
	for _, f := range fields {
		if err := CheckPath(f.Path); err != nil {
			return err
		}
		if !namePattern().MatchString(f.Name) || seen[f.Name] {
			return fmt.Errorf("%w: field name %q", ErrInvalidPath, f.Name)
		}
		seen[f.Name] = true
	}
	return nil
}

/*
CheckPath validates one path.
*/
func CheckPath(path string) error {
	if len(path) > maxPath || !pathPattern().MatchString(path) {
		return fmt.Errorf("%w: %q", ErrInvalidPath, path)
	}
	return nil
}

/*
Shape maps a JSON body to the result the model reads. With no fields
the whole body is kept, capped like any result.
*/
func Shape(body []byte, fields []ResponseField, maxBytes int) (json.RawMessage, error) {
	if !gjson.ValidBytes(body) {
		return nil, ErrInvalidResponse
	}
	var value any
	if len(fields) == 0 {
		value = gjson.ParseBytes(body).Value()
	} else {
		picked := map[string]any{}
		for _, f := range fields {
			if r := gjson.GetBytes(body, f.Path); r.Exists() {
				picked[f.Name] = r.Value()
			}
		}
		value = picked
	}
	return Cap(value, maxBytes), nil
}

/*
Text wraps a plain-text result for the model, cut to a few kilobytes
and with secrets masked.
*/
func Text(text string) json.RawMessage {
	out, err := json.Marshal(map[string]any{"text": pii.New().RedactSecrets(truncate(text, maxTextChars))})
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return out
}

/*
Cap cuts lists, masks secrets and keeps the encoded result under
maxBytes, cutting lists harder before giving up on the content.
*/
func Cap(value any, maxBytes int) json.RawMessage {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBytes
	}
	r := pii.New()
	for _, limit := range []int{maxArray, smallArray} {
		out, err := json.Marshal(scrub(r, value, limit))
		if err == nil && len(out) <= maxBytes {
			return out
		}
	}
	return json.RawMessage(`{"_truncated":true,"note":"the result was too large to show"}`)
}

/*
ErrorMessage reads the upstream's own error message at the path, cut to
a short line, empty when there is none.
*/
func ErrorMessage(body []byte, path string) string {
	if path == "" || CheckPath(path) != nil || !gjson.ValidBytes(body) {
		return ""
	}
	r := gjson.GetBytes(body, path)
	if !r.Exists() {
		return ""
	}
	return pii.New().RedactSecrets(truncate(strings.TrimSpace(r.String()), maxErrorChars))
}

/*
scrub walks a value, cutting lists to the limit and masking secrets in
every string, keeping the emails and phone numbers a customer may need
to hear back.
*/
func scrub(r *pii.Redactor, v any, limit int) any {
	switch t := v.(type) {
	case string:
		return r.RedactSecrets(t)
	case []any:
		if len(t) <= limit {
			out := make([]any, len(t))
			for i, item := range t {
				out[i] = scrub(r, item, limit)
			}
			return out
		}
		out := make([]any, 0, limit+1)
		for _, item := range t[:limit] {
			out = append(out, scrub(r, item, limit))
		}
		return append(out, map[string]any{"_truncated": true, "_total": len(t)})
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, item := range t {
			out[k] = scrub(r, item, limit)
		}
		return out
	}
	return v
}

/*
truncate keeps at most n runes.
*/
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}
