/*
|--------------------------------------------------------------------------
| Local Signed URLs
|--------------------------------------------------------------------------
|
| A local disk signs temporary URLs itself. The signature is an HMAC over
| the method, the path, the expiry and any pinned content type, so a
| download link cannot be turned into an upload, moved to another file,
| extended, or used to upload a different type. The application serves
| the links under the configured base and calls Verify before touching
| the disk.
|
*/

package local

import (
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/hashing"
)

/*
Errors from checking a signed request.

ErrInvalidSignature means the link was not signed with the key for this
request: a changed path, method, expiry or content type, or a missing
signature. ErrExpired means the link was valid but its time has passed.
*/
var (
	ErrInvalidSignature = errors.New("local disk: invalid signature")
	ErrExpired          = errors.New("local disk: signed url expired")
)

const (
	queryExpires     = "expires"
	queryContentType = "content_type"
	querySignature   = "signature"
	queryFilename    = "filename"
)

type signer struct {
	base *url.URL
	key  []byte
}

func (s *signer) sign(name string, req filesystem.SignRequest, now time.Time) string {
	expires := strconv.FormatInt(now.Add(req.TTL).Unix(), 10)
	contentType := ""

	query := url.Values{queryExpires: {expires}}
	if req.Method == http.MethodPut && req.ContentType != "" {
		contentType = req.ContentType
		query.Set(queryContentType, contentType)
	}

	fields := []string{req.Method, name, expires, contentType}
	if req.Method == http.MethodGet && req.Filename != "" {
		query.Set(queryFilename, req.Filename)
		fields = append(fields, req.Filename)
	}

	query.Set(querySignature, hex.EncodeToString(signature(s.key, fields...)))

	u := s.base.JoinPath(name)
	u.RawQuery = query.Encode()

	return u.String()
}

/*
Verify checks a request made to a temporary URL of a local disk with the
disk's signing key. The path is the file path the route took from the
URL, unescaped, and now is the current time. HEAD is checked as GET. An
upload link with a pinned content type accepts only that Content-Type.
*/
func Verify(r *http.Request, path, signingKey string, now time.Time) error {
	query := r.URL.Query()
	expires := query.Get(queryExpires)

	deadline, err := strconv.ParseInt(expires, 10, 64)
	if err != nil {
		return ErrInvalidSignature
	}

	method := r.Method
	if method == http.MethodHead {
		method = http.MethodGet
	}

	contentType := query.Get(queryContentType)
	if contentType != "" && (method != http.MethodPut || r.Header.Get("Content-Type") != contentType) {
		return ErrInvalidSignature
	}

	fields := []string{method, path, expires, contentType}
	if filename := query.Get(queryFilename); filename != "" {
		fields = append(fields, filename)
	}

	got, err := hex.DecodeString(query.Get(querySignature))
	if err != nil || !hashing.Equal(got, signature([]byte(signingKey), fields...)) {
		return ErrInvalidSignature
	}

	if now.Unix() > deadline {
		return ErrExpired
	}

	return nil
}

/*
signature is the HMAC over the method, path, expiry, content type and,
for a named download, the name, one per line.
*/
func signature(key []byte, fields ...string) []byte {
	return hashing.HMAC(key, []byte(strings.Join(fields, "\n")))
}
