/*
|--------------------------------------------------------------------------
| Security Headers
|--------------------------------------------------------------------------
|
| Browsers enforce a set of protections only when the response asks for
| them: no content sniffing, no framing by other sites, no full URLs in
| referrers, HTTPS only, and a content security policy. A policy that
| allows inline scripts by nonce gets a fresh nonce per request, which
| templates read from the context.
|
*/

package middleware

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"time"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/view"
)

/*
NoncePlaceholder is replaced by the request's nonce in the content
security policy.
*/
const NoncePlaceholder = "{nonce}"

/*
SecurityHeadersOptions configure the security headers.

ContentSecurityPolicy is sent as is, with NoncePlaceholder replaced by a
fresh nonce; empty sends none. FrameOptions defaults to DENY and
ReferrerPolicy to strict-origin-when-cross-origin. HSTSMaxAge enables
Strict-Transport-Security when positive. PermissionsPolicy is sent when
set.
*/
type SecurityHeadersOptions struct {
	ContentSecurityPolicy string
	FrameOptions          string
	ReferrerPolicy        string
	HSTSMaxAge            time.Duration
	PermissionsPolicy     string
}

/*
SecurityHeaders sets the security headers on every response.
*/
func SecurityHeaders(opts SecurityHeadersOptions) fhttp.Middleware {
	static := staticHeaders(opts)
	withNonce := strings.Contains(opts.ContentSecurityPolicy, NoncePlaceholder)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for name, value := range static {
				w.Header().Set(name, value)
			}

			if withNonce {
				nonce := newNonce()
				w.Header().Set("Content-Security-Policy",
					strings.ReplaceAll(opts.ContentSecurityPolicy, NoncePlaceholder, nonce))
				r = r.WithContext(view.WithNonce(r.Context(), nonce))
			}

			next.ServeHTTP(w, r)
		})
	}
}

func staticHeaders(opts SecurityHeadersOptions) map[string]string {
	headers := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        orDefault(opts.FrameOptions, "DENY"),
		"Referrer-Policy":        orDefault(opts.ReferrerPolicy, "strict-origin-when-cross-origin"),
	}

	if opts.HSTSMaxAge > 0 {
		headers["Strict-Transport-Security"] = "max-age=" + strconv.Itoa(int(opts.HSTSMaxAge/time.Second)) +
			"; includeSubDomains"
	}

	if opts.PermissionsPolicy != "" {
		headers["Permissions-Policy"] = opts.PermissionsPolicy
	}

	if opts.ContentSecurityPolicy != "" && !strings.Contains(opts.ContentSecurityPolicy, NoncePlaceholder) {
		headers["Content-Security-Policy"] = opts.ContentSecurityPolicy
	}

	return headers
}

/*
newNonce returns 16 random bytes in base64. The system's random source
does not fail on supported platforms.
*/
func newNonce() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)

	return base64.StdEncoding.EncodeToString(b)
}

func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}

	return value
}
