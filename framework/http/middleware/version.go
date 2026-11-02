/*
|--------------------------------------------------------------------------
| API and Client Versions
|--------------------------------------------------------------------------
|
| Every API response names the API version that produced it, and a client
| may ask for a version: one the server does not speak is refused rather
| than answered in a shape the client cannot read. Apps that announce
| their own version are told to upgrade once they fall below the minimum
| for their platform. Clients that announce nothing, such as browsers,
| are never refused. The header names are the application's.
|
*/

package middleware

import (
	"net/http"
	"slices"
	"strconv"
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"
)

/*
APIVersion writes the current version in the header of every response
and refuses requests asking for a version that is neither current nor
supported with 400 invalid_api_version.
*/
func APIVersion(header, current string, supported []string) fhttp.Middleware {
	accepted := append([]string{current}, supported...)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(header, current)

			asked := r.Header.Get(header)
			if asked != "" && !slices.Contains(accepted, asked) {
				refuse(w, r, &fhttp.ResponseError{
					Status:  http.StatusBadRequest,
					Message: "This API version is not supported.",
					Code:    "invalid_api_version",
					Meta:    map[string]any{"supported_versions": accepted},
				})

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

/*
ClientVersion reads "platform/version" from the header, such as
"ios/1.4.0", and refuses clients below the minimum version for their
platform with 426 client_upgrade_required, naming the minimum and the
platform's upgrade URL. A missing or malformed header is let through.
*/
func ClientVersion(header string, minimums, upgradeURLs map[string]string) fhttp.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			platform, version, ok := strings.Cut(r.Header.Get(header), "/")
			minimum, limited := minimums[platform]

			if !ok || !limited || !older(version, minimum) {
				next.ServeHTTP(w, r)

				return
			}

			refuse(w, r, &fhttp.ResponseError{
				Status:  http.StatusUpgradeRequired,
				Message: "This version of the app is no longer supported. Please upgrade.",
				Code:    "client_upgrade_required",
				Meta:    map[string]any{"min_version": minimum, "upgrade_url": upgradeURLs[platform]},
			})
		})
	}
}

/*
older reports whether the version is below the minimum. Versions that
do not parse are never older, so a malformed header is ignored.
*/
func older(version, minimum string) bool {
	got, ok := parseVersion(version)
	if !ok {
		return false
	}

	want, ok := parseVersion(minimum)
	if !ok {
		return false
	}

	return slices.Compare(got, want) < 0
}

/*
parseVersion reads major.minor.patch, with minor and patch optional and
any pre-release or build suffix ignored.
*/
func parseVersion(s string) ([]int, bool) {
	core, _, _ := strings.Cut(s, "-")
	core, _, _ = strings.Cut(core, "+")

	parts := strings.Split(core, ".")
	if len(parts) > 3 {
		return nil, false
	}

	out := make([]int, 3)
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return nil, false
		}

		out[i] = n
	}

	return out, true
}
