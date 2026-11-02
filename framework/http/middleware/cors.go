/*
|--------------------------------------------------------------------------
| Cross-Origin Resource Sharing
|--------------------------------------------------------------------------
|
| Browsers on other sites may call the API only as the configuration
| allows. Origins are compared after normalizing case, default ports and
| international names, and a wildcard covers exactly one label, so a
| lookalike or a public suffix never slips through.
|
*/

package middleware

import (
	"slices"
	"time"

	"github.com/rs/cors"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/origin"
)

/*
CORSOptions configure cross-origin access.

AllowedOrigins lists origin patterns such as "https://*.example.com", or
"*" for any origin. AllowedMethods, AllowedHeaders and ExposedHeaders
list what a cross-origin request may use and read. AllowCredentials lets
cookies through, which never combines with "*". MaxAge is how long a
browser may cache a preflight answer.
*/
type CORSOptions struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           time.Duration
}

/*
CORS answers preflight requests and marks responses readable by the
allowed origins.
*/
func CORS(opts CORSOptions) fhttp.Middleware {
	options := cors.Options{
		AllowedMethods:   opts.AllowedMethods,
		AllowedHeaders:   opts.AllowedHeaders,
		ExposedHeaders:   opts.ExposedHeaders,
		AllowCredentials: opts.AllowCredentials,
		MaxAge:           int(opts.MaxAge / time.Second),
	}

	if slices.Contains(opts.AllowedOrigins, "*") {
		options.AllowedOrigins = []string{"*"}
	} else {
		patterns := slices.Clone(opts.AllowedOrigins)
		options.AllowOriginFunc = func(raw string) bool { return allowed(patterns, raw) }
	}

	return cors.New(options).Handler
}

func allowed(patterns []string, raw string) bool {
	o, err := origin.Parse(raw)
	if err != nil {
		return false
	}

	return slices.ContainsFunc(patterns, func(pattern string) bool { return origin.Match(pattern, o) })
}
