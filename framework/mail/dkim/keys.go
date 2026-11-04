/*
|--------------------------------------------------------------------------
| Signing Keys
|--------------------------------------------------------------------------
|
| A domain we sign mail for gets a key of its own: RSA of 2048 bits,
| which every receiver accepts and fits a DNS record once split. The
| private half is handed back as PEM for safe keeping, the public half in
| the base64 form its DKIM record publishes.
|
*/

package dkim

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
)

/*
keyBits is the size of the keys GenerateKey makes.
*/
const keyBits = 2048

/*
Key is a signing key: the PEM encoded PKCS#8 private half, and the
public half as its record's p= value.
*/
type Key struct {
	PrivatePEM string
	Public     string
}

/*
GenerateKey makes a new signing key.
*/
func GenerateKey() (Key, error) {
	private, err := rsa.GenerateKey(rand.Reader, keyBits)
	if err != nil {
		return Key{}, fmt.Errorf("dkim: generate key: %w", err)
	}

	der, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		return Key{}, fmt.Errorf("dkim: encode key: %w", err)
	}

	public, err := publicValue(private)
	if err != nil {
		return Key{}, err
	}

	block := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})

	return Key{PrivatePEM: string(block), Public: public}, nil
}

/*
PublicKey returns the record value of a PEM private key's public half.
*/
func PublicKey(privatePEM string) (string, error) {
	key, err := parseKey(privatePEM)
	if err != nil {
		return "", err
	}

	return publicValue(key)
}

/*
publicValue encodes a key's public half as its record publishes it.
*/
func publicValue(key crypto.Signer) (string, error) {
	der, err := x509.MarshalPKIXPublicKey(key.Public())
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrInvalidKey, err)
	}

	return base64.StdEncoding.EncodeToString(der), nil
}
