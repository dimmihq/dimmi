/*
|--------------------------------------------------------------------------
| Signed URLs
|--------------------------------------------------------------------------
|
| A signed URL carries its own proof that the application issued it, so
| a link in an email can unsubscribe or download without a session. The
| signature covers the path and every query parameter, so nothing in the
| link can be changed, and an expiry can be part of what is signed. The
| host is not signed: the same link verifies behind any proxy or domain
| that routes to the application.
|
*/

package routing

import (
	"encoding/hex"
	"net/url"
	"strconv"
	"time"

	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/http"
)

/*
The query parameters a signed URL carries.
*/
const (
	ExpiresParameter   = "expires"
	SignatureParameter = "signature"
)

/*
SignedRoute returns the absolute URL of the named route with a signature.
A positive ttl makes the link expire that long from now; zero makes it
valid until the key changes.
*/
func (u *URL) SignedRoute(name string, params map[string]string, ttl time.Duration) (string, error) {
	if len(u.key) == 0 {
		return "", ErrNoSigningKey
	}

	path, query, err := u.fill(name, params)
	if err != nil {
		return "", err
	}

	query.Del(SignatureParameter)
	if ttl > 0 {
		query.Set(ExpiresParameter, strconv.FormatInt(u.now().Add(ttl).Unix(), 10))
	}

	unsigned := withQuery(path, query)
	query.Set(SignatureParameter, u.sign(unsigned))

	return u.base + withQuery(path, query), nil
}

/*
HasValidSignature reports whether the request's URL carries a signature
made with the key and, when it has an expiry, has not expired.
*/
func (u *URL) HasValidSignature(r *http.Request) bool {
	if len(u.key) == 0 {
		return false
	}

	query := r.URL.Query()
	signature := query.Get(SignatureParameter)
	query.Del(SignatureParameter)

	got, err := hex.DecodeString(signature)
	if err != nil || signature == "" {
		return false
	}

	want := hashing.HMAC(u.key, []byte(withQuery(r.URL.EscapedPath(), query)))
	if !hashing.Equal(got, want) {
		return false
	}

	return !u.expired(query)
}

func (u *URL) expired(query url.Values) bool {
	raw := query.Get(ExpiresParameter)
	if raw == "" {
		return false
	}

	expires, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return true
	}

	return u.now().Unix() > expires
}

func (u *URL) sign(unsigned string) string {
	return hex.EncodeToString(hashing.HMAC(u.key, []byte(unsigned)))
}
