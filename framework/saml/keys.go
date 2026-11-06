/*
|--------------------------------------------------------------------------
| Service Provider Keys
|--------------------------------------------------------------------------
|
| Every connection to an identity provider gets a key pair of its own, so
| replacing one company's connection never touches another's. The
| certificate is self-signed: identity providers trust it because an
| administrator pasted it in, not because of who issued it. It lasts ten
| years, since a certificate expiring silently breaks every sign-in.
|
*/

package saml

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"time"
)

/*
keyBits is the size of a service provider's RSA key.
*/
const keyBits = 2048

/*
certificateLife is how long a service provider certificate is valid.
*/
const certificateLife = 10 * 365 * 24 * time.Hour

/*
ErrInvalidKey means a PEM block holds no key or certificate this
package can use.
*/
var ErrInvalidKey = errors.New("saml: invalid key or certificate")

/*
KeyPair is a service provider's signing key and its certificate, both
PEM encoded.
*/
type KeyPair struct {
	PrivateKey  string
	Certificate string
}

/*
NewKeyPair makes a key pair for the service provider named by its entity
id, its certificate valid from now.
*/
func NewKeyPair(entityID string, now time.Time) (KeyPair, error) {
	key, err := rsa.GenerateKey(rand.Reader, keyBits)
	if err != nil {
		return KeyPair{}, fmt.Errorf("generate service provider key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 127))
	if err != nil {
		return KeyPair{}, fmt.Errorf("generate certificate serial: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: entityID},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(certificateLife),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return KeyPair{}, fmt.Errorf("sign service provider certificate: %w", err)
	}

	private := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})

	return KeyPair{
		PrivateKey:  string(private),
		Certificate: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})),
	}, nil
}

/*
ParseCertificate reads a PEM certificate, or its base64 body alone as
identity provider metadata carries it.
*/
func ParseCertificate(text string) (*x509.Certificate, error) {
	block, _ := pem.Decode([]byte(text))
	if block == nil {
		block, _ = pem.Decode([]byte("-----BEGIN CERTIFICATE-----\n" + text + "\n-----END CERTIFICATE-----\n"))
	}

	if block == nil {
		return nil, ErrInvalidKey
	}

	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidKey, err)
	}

	return certificate, nil
}

/*
ParsePrivateKey reads the PEM private key a key pair holds.
*/
func ParsePrivateKey(text string) (crypto.Signer, error) {
	block, _ := pem.Decode([]byte(text))
	if block == nil {
		return nil, ErrInvalidKey
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidKey, err)
	}

	return key, nil
}

/*
ParseCertificates reads every certificate of the list, refusing the list
when one does not read.
*/
func ParseCertificates(texts []string) ([]*x509.Certificate, error) {
	certificates := make([]*x509.Certificate, 0, len(texts))

	for _, text := range texts {
		certificate, err := ParseCertificate(text)
		if err != nil {
			return nil, err
		}

		certificates = append(certificates, certificate)
	}

	return certificates, nil
}

/*
EncodeCertificate writes the certificate PEM encoded.
*/
func EncodeCertificate(certificate *x509.Certificate) string {
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Raw}))
}
