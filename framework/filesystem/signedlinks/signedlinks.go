/*
|--------------------------------------------------------------------------
| App Signed Links
|--------------------------------------------------------------------------
|
| A disk the application serves itself signs its temporary URLs with a
| key of the application's: a local disk always, and a storage service's
| disk when browsers cannot reach the service. The signature is an HMAC
| over the method, the path, the expiry, any pinned content type and any
| download name, so a download link cannot be turned into an upload,
| moved to another file, extended, renamed, or used to upload a
| different type. The application serves the links under the configured
| base and calls Verify before touching the disk.
|
*/

package signedlinks

import (
	"encoding/hex"
	"errors"
	"fmt"
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
	ErrInvalidSignature = errors.New("signed link: invalid signature")
	ErrExpired          = errors.New("signed link: expired")
)

const (
	queryExpires     = "expires"
	queryContentType = "content_type"
	querySignature   = "signature"
	queryFilename    = "filename"
)

/*
Signer signs links to the files of one disk, under the base the
application serves them from.
*/
type Signer struct {
	base *url.URL
	key  []byte
}

/*
New returns a signer for links under the base, signed with the key.
*/
func New(base, key string) (*Signer, error) {
	parsed, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("parse signed_url_base: %w", err)
	}

	return &Signer{base: parsed, key: []byte(key)}, nil
}

/*
Sign returns the link that makes the request of the named file until
its TTL passes from now.
*/
func (s *Signer) Sign(name string, req filesystem.SignRequest, now time.Time) string {
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
