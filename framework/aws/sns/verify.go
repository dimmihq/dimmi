/*
|--------------------------------------------------------------------------
| Signature Verification
|--------------------------------------------------------------------------
|
| A message is trusted only when it is signed by the certificate SNS
| names, that certificate comes over https from an SNS host of AWS, and
| the message is for a topic the application subscribed to. Checking the
| host matters as much as the signature: anyone can sign a message with
| a certificate they host themselves.
|
| Certificates rarely change and are named by URL, so each one is
| fetched once and kept for the life of the verifier.
|
*/

package sns

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"

	"github.com/dimmihq/dimmi/framework/httpclient"
)

/*
Errors from verification.

ErrTopicNotAllowed rejects a message for a topic the caller did not
allow. ErrCertificate rejects a signing certificate that is not served
by SNS over https or cannot be read. ErrSignature means the signature
does not match the message.
*/
var (
	ErrTopicNotAllowed = errors.New("sns: topic not allowed")
	ErrCertificate     = errors.New("sns: signing certificate not trusted")
	ErrSignature       = errors.New("sns: signature does not match")
)

/*
maxCertificateSize caps the certificate download.
*/
const maxCertificateSize = 64 << 10

/*
Allow lists the topic ARNs whose messages are accepted. An empty list
accepts none.
*/
type Allow struct {
	TopicARNs []string
}

/*
Verifier checks SNS message signatures. It is safe for concurrent use.
*/
type Verifier struct {
	client *httpclient.Client

	mu    sync.Mutex
	certs map[string]*x509.Certificate
}

/*
NewVerifier returns a verifier that fetches certificates through the
client.
*/
func NewVerifier(client *httpclient.Client) *Verifier {
	return &Verifier{client: client, certs: map[string]*x509.Certificate{}}
}

/*
Verify returns nil only for a message SNS signed, for an allowed topic.
Signature version 1 signs with SHA1 and version 2 with SHA256.
*/
func (v *Verifier) Verify(ctx context.Context, msg Message, allow Allow) error {
	if !slices.Contains(allow.TopicARNs, msg.TopicARN) {
		return fmt.Errorf("%w: %s", ErrTopicNotAllowed, msg.TopicARN)
	}

	algorithm, err := algorithmFor(msg.SignatureVersion)
	if err != nil {
		return err
	}

	text, err := signedText(msg)
	if err != nil {
		return err
	}

	signature, err := base64.StdEncoding.DecodeString(msg.Signature)
	if err != nil {
		return fmt.Errorf("%w: signature is not base64", ErrSignature)
	}

	cert, err := v.certificate(ctx, msg.SigningCertURL)
	if err != nil {
		return err
	}

	if err := cert.CheckSignature(algorithm, []byte(text), signature); err != nil {
		return fmt.Errorf("%w: %w", ErrSignature, err)
	}

	return nil
}

func algorithmFor(version string) (x509.SignatureAlgorithm, error) {
	switch version {
	case "1":
		return x509.SHA1WithRSA, nil
	case "2":
		return x509.SHA256WithRSA, nil
	default:
		return 0, fmt.Errorf("%w: signature version %q", ErrInvalidMessage, version)
	}
}

func (v *Verifier) certificate(ctx context.Context, certURL string) (*x509.Certificate, error) {
	if !trustedCertURL(certURL) {
		return nil, fmt.Errorf("%w: %s", ErrCertificate, certURL)
	}

	v.mu.Lock()
	cert, ok := v.certs[certURL]
	v.mu.Unlock()

	if ok {
		return cert, nil
	}

	cert, err := v.fetch(ctx, certURL)
	if err != nil {
		return nil, err
	}

	v.mu.Lock()
	v.certs[certURL] = cert
	v.mu.Unlock()

	return cert, nil
}

func (v *Verifier) fetch(ctx context.Context, certURL string) (*x509.Certificate, error) {
	resp, err := v.client.New().MaxBody(maxCertificateSize).WithoutRedirects().Get(ctx, certURL)
	if err != nil {
		return nil, fmt.Errorf("fetch signing certificate: %w", err)
	}

	if resp.Status != http.StatusOK {
		return nil, fmt.Errorf("%w: fetch returned status %d", ErrCertificate, resp.Status)
	}

	block, _ := pem.Decode(resp.Body)
	if block == nil {
		return nil, fmt.Errorf("%w: not a pem certificate", ErrCertificate)
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCertificate, err)
	}

	return cert, nil
}

/*
trustedCertURL accepts only https URLs on an SNS regional host of AWS,
such as sns.us-east-1.amazonaws.com, ending in .pem.
*/
func trustedCertURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Port() != "" || !strings.HasSuffix(u.Path, ".pem") {
		return false
	}

	region, ok := strings.CutPrefix(u.Hostname(), "sns.")
	if !ok {
		return false
	}

	region, ok = strings.CutSuffix(region, ".amazonaws.com")
	if !ok {
		region, ok = strings.CutSuffix(strings.TrimSuffix(region, ".cn"), ".amazonaws.com")
	}

	return ok && region != "" && strings.Trim(region, "abcdefghijklmnopqrstuvwxyz0123456789-") == ""
}
