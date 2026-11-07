/*
|--------------------------------------------------------------------------
| Derived VAPID Keys
|--------------------------------------------------------------------------
|
| Push works without anyone generating keys: when none are configured,
| the key pair is derived from a secret the install already has, so every
| server of the install signs with the same pair and it survives
| restarts. Configured keys always win. A new secret means a new pair,
| and browsers then subscribe again.
|
*/

package webpush

import (
	"crypto/ecdh"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

/*
Defaults fill what the configuration leaves empty: Seed derives the key
pair and Subject is where push services reach the sender.
*/
type Defaults struct {
	Seed    []byte
	Subject string
}

/*
ErrNoSeed refuses to derive keys from an empty secret.
*/
var ErrNoSeed = errors.New("webpush: no seed to derive keys from")

/*
DeriveKeys returns the VAPID key pair belonging to the seed, base64url
encoded: the same seed always gives the same pair.
*/
func DeriveKeys(seed []byte) (privateKey, publicKey string, err error) {
	if len(seed) == 0 {
		return "", "", ErrNoSeed
	}
	for counter := byte(0); ; counter++ {
		material := sha256.Sum256(append(append([]byte("webpush-vapid:"), seed...), counter))
		key, err := ecdh.P256().NewPrivateKey(material[:])
		if err != nil {
			continue
		}
		encode := base64.RawURLEncoding.EncodeToString
		return encode(key.Bytes()), encode(key.PublicKey().Bytes()), nil
	}
}

/*
withDefaults fills the configuration's empty key pair and subject.
*/
func (c Config) withDefaults(d Defaults) (Config, error) {
	if c.VAPIDPrivateKey == "" && c.VAPIDPublicKey == "" && len(d.Seed) > 0 {
		private, public, err := DeriveKeys(d.Seed)
		if err != nil {
			return c, fmt.Errorf("derive vapid keys: %w", err)
		}
		c.VAPIDPrivateKey, c.VAPIDPublicKey = private, public
	}
	if c.Subject == "" {
		c.Subject = subjectFrom(d.Subject)
	}
	return c, nil
}

/*
subjectFrom turns the default subject into one push services accept:
an https address or a mailto: address as it is, anything else, such as
a plain http address in development, a mailto: address that reaches
nobody.
*/
func subjectFrom(subject string) string {
	if strings.HasPrefix(subject, "https:") || strings.HasPrefix(subject, "mailto:") {
		return subject
	}
	return "mailto:webpush@localhost"
}
