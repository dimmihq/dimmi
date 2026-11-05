/*
|--------------------------------------------------------------------------
| Signed Inbound Parse Posts
|--------------------------------------------------------------------------
|
| SendGrid signs each post with its own P-256 key over the timestamp and
| the body. Tests of the application hold a key of their own, hand its
| public half to the source as the install would, and sign real
| multipart bodies, so every check the source makes runs for real.
|
*/

package sendgridtest

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"maps"
	"mime/multipart"
	"slices"
	"testing"
)

/*
Signer signs posts with one key.
*/
type Signer struct {
	key    *ecdsa.PrivateKey
	public string
}

/*
Post is a signed post: its body, content type and signature headers.
*/
type Post struct {
	Body        []byte
	ContentType string
	Timestamp   string
	Signature   string
}

/*
NewSigner returns a signer with a fresh key.
*/
func NewSigner(tb testing.TB) *Signer {
	tb.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	check(tb, err)

	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	check(tb, err)

	return &Signer{key: key, public: base64.StdEncoding.EncodeToString(der)}
}

/*
PublicKey returns the verification key as SendGrid shows it: base64 of
the DER public key.
*/
func (s *Signer) PublicKey() string {
	return s.public
}

/*
Sign builds a multipart body from the fields, in name order, and signs it
for the timestamp.
*/
func (s *Signer) Sign(tb testing.TB, timestamp string, fields map[string]string) Post {
	tb.Helper()

	var body bytes.Buffer

	writer := multipart.NewWriter(&body)
	for _, name := range slices.Sorted(maps.Keys(fields)) {
		check(tb, writer.WriteField(name, fields[name]))
	}

	check(tb, writer.Close())

	digest := sha256.Sum256(append([]byte(timestamp), body.Bytes()...))

	signature, err := ecdsa.SignASN1(rand.Reader, s.key, digest[:])
	check(tb, err)

	return Post{
		Body: body.Bytes(), ContentType: writer.FormDataContentType(), Timestamp: timestamp,
		Signature: base64.StdEncoding.EncodeToString(signature),
	}
}

func check(tb testing.TB, err error) {
	tb.Helper()

	if err != nil {
		tb.Fatalf("sendgridtest: %v", err)
	}
}
