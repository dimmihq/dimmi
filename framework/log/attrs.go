/*
|--------------------------------------------------------------------------
| Attributes
|--------------------------------------------------------------------------
|
| A line's details are typed attributes, never loose key and value pairs,
| so a typo cannot shift every value onto the wrong key. Keys are
| snake_case. Errors always use the key "error", so every failure can be
| found with one search.
|
*/

package log

import (
	"log/slog"
	"time"
)

/*
Attr is one key and value on a line.
*/
type Attr = slog.Attr

/*
String returns a string attribute.
*/
func String(key, value string) Attr {
	return slog.String(key, value)
}

/*
Int returns an integer attribute.
*/
func Int(key string, value int) Attr {
	return slog.Int(key, value)
}

/*
Int64 returns a 64-bit integer attribute.
*/
func Int64(key string, value int64) Attr {
	return slog.Int64(key, value)
}

/*
Float64 returns a floating-point attribute.
*/
func Float64(key string, value float64) Attr {
	return slog.Float64(key, value)
}

/*
Bool returns a boolean attribute.
*/
func Bool(key string, value bool) Attr {
	return slog.Bool(key, value)
}

/*
Duration returns a duration attribute.
*/
func Duration(key string, value time.Duration) Attr {
	return slog.Duration(key, value)
}

/*
Time returns a time attribute.
*/
func Time(key string, value time.Time) Attr {
	return slog.Time(key, value)
}

/*
Any returns an attribute for any value. Structs and maps are walked by
redaction, so their secret fields are hidden too.
*/
func Any(key string, value any) Attr {
	return slog.Any(key, value)
}

/*
Group returns an attribute that nests the attributes under the key.
*/
func Group(key string, attrs ...Attr) Attr {
	return slog.GroupAttrs(key, attrs...)
}

/*
Err returns the error as the attribute "error".
*/
func Err(err error) Attr {
	return slog.Any("error", err)
}
