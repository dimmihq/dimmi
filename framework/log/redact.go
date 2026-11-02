/*
|--------------------------------------------------------------------------
| Redaction
|--------------------------------------------------------------------------
|
| Logs are copied to places with weaker access control than the database,
| so secrets must never be written to them. A value is hidden when its
| attribute key or struct field is named like a secret, matched without
| regard to case or underscores so api_key and APIKey are the same name,
| or when its struct field is tagged `masq:"secret"`. Redaction walks
| nested structs, maps and slices.
|
*/

package log

import (
	"log/slog"
	"slices"
	"strings"

	"github.com/m-mizutani/masq"
)

/*
secretNames lists the names that always hold secrets.
*/
func secretNames() []string {
	return []string{
		"password", "passwd", "secret", "client_secret", "token",
		"access_token", "refresh_token", "id_token", "api_key",
		"authorization", "cookie", "set_cookie", "private_key",
	}
}

/*
redactor returns the slog ReplaceAttr hook that hides the built-in
secret names, the extra names and every field tagged as secret.

Time values pass through untouched. They never hold secrets, and copying
one reads the time zone the runtime loads lazily, which races with other
goroutines logging at the same moment.
*/
func redactor(extra []string) func(groups []string, attr slog.Attr) slog.Attr {
	names := map[string]struct{}{}
	for _, name := range slices.Concat(secretNames(), extra) {
		names[normalize(name)] = struct{}{}
	}

	isSecret := func(field string, _ any, _ string) bool {
		_, ok := names[normalize(field)]

		return ok
	}

	redact := masq.New(masq.WithTag("secret"), masq.WithCensor(isSecret))

	return func(groups []string, attr slog.Attr) slog.Attr {
		if attr.Value.Kind() == slog.KindTime {
			return attr
		}

		return redact(groups, attr)
	}
}

func normalize(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, "_", ""))
}
