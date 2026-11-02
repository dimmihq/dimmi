/*
|--------------------------------------------------------------------------
| DKIM Signing
|--------------------------------------------------------------------------
|
| A relay sends mail as whatever domain the message says, so receivers
| trust a sender's domain only when the message carries a signature made
| with a key that domain publishes in DNS. Providers sign for us; a plain
| relay does not, so messages sent through one are signed here first.
|
| Only header fields that identify the message and its threading are
| signed. Relays add their own fields on the way, and signing those
| would break the signature for no gain.
|
*/

package dkim

import (
	"bytes"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"

	"github.com/emersion/go-msgauth/dkim"
)

/*
ErrInvalidKey rejects a private key that is not a PEM encoded RSA or
Ed25519 key.
*/
var ErrInvalidKey = errors.New("dkim: invalid private key")

/*
Options configure a signer: the signing domain, the selector under which
the public key is published, and the PEM encoded private key, PKCS#1 or
PKCS#8.
*/
type Options struct {
	Domain     string
	Selector   string
	PrivateKey string
}

/*
Signer adds DKIM signatures to messages. It is safe for concurrent use.
*/
type Signer struct {
	domain   string
	selector string
	key      crypto.Signer
}

/*
New returns a signer for the options.
*/
func New(opts Options) (*Signer, error) {
	if opts.Domain == "" || opts.Selector == "" {
		return nil, fmt.Errorf("%w: domain and selector are required", ErrInvalidKey)
	}

	key, err := parseKey(opts.PrivateKey)
	if err != nil {
		return nil, err
	}

	return &Signer{domain: opts.Domain, selector: opts.Selector, key: key}, nil
}

/*
Sign returns the message with a DKIM-Signature header field in front.
*/
func (s *Signer) Sign(raw []byte) ([]byte, error) {
	var signed bytes.Buffer

	err := dkim.Sign(&signed, bytes.NewReader(raw), &dkim.SignOptions{
		Domain:                 s.domain,
		Selector:               s.selector,
		Signer:                 s.key,
		HeaderCanonicalization: dkim.CanonicalizationRelaxed,
		BodyCanonicalization:   dkim.CanonicalizationRelaxed,
		HeaderKeys:             signedHeaders(),
	})
	if err != nil {
		return nil, fmt.Errorf("dkim sign: %w", err)
	}

	return signed.Bytes(), nil
}

/*
signedHeaders are the header fields a signature covers, when present.
*/
func signedHeaders() []string {
	return []string{
		"From", "Reply-To", "To", "Cc", "Subject", "Date", "Message-ID",
		"In-Reply-To", "References", "MIME-Version", "Content-Type",
		"List-Unsubscribe", "List-Unsubscribe-Post", "List-Id",
	}
}

func parseKey(text string) (crypto.Signer, error) {
	block, _ := pem.Decode([]byte(text))
	if block == nil {
		return nil, fmt.Errorf("%w: no pem block", ErrInvalidKey)
	}

	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidKey, err)
	}

	key, ok := parsed.(crypto.Signer)
	if !ok {
		return nil, fmt.Errorf("%w: unsupported key type %T", ErrInvalidKey, parsed)
	}

	return key, nil
}
