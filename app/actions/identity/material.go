/*
|--------------------------------------------------------------------------
| Identity Secret Material
|--------------------------------------------------------------------------
|
| A shared secret is dmi_idsec_ and 43 random letters and digits, about
| 256 bits, and the HMAC key is the whole string as typed, so a developer never
| decodes anything. It is sealed to its row and shown once. A public key
| arrives as PEM and only its public half is ever accepted, so a host
| that pastes its private key by mistake is told rather than served.
|
*/

package identity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/jwt"
	"github.com/dimmihq/dimmi/framework/support/str"

	"github.com/dimmihq/dimmi/app/models"
)

/*
MaterialPrefix starts every shared secret, so log redaction and secret
scanners recognize one.
*/
const MaterialPrefix = "dmi_idsec_"

/*
The algorithms a secret may use.
*/
const (
	AlgorithmHS256 = "HS256"
	AlgorithmRS256 = "RS256"
	AlgorithmEdDSA = "EdDSA"
)

/*
publicKey is how a public key is stored: its PEM text.
*/
type publicKey struct {
	PEM string `json:"pem"`
}

/*
newSharedSecret returns a fresh shared secret.
*/
func newSharedSecret() string {
	return MaterialPrefix + str.Random(43)
}

/*
secretSlot is where a shared secret is sealed: the workspace's scope,
bound to its row.
*/
func secretSlot(workspaceID, id ids.UUID) encryption.Slot {
	return encryption.Slot{
		Scope: encryption.Scope{Kind: "workspace", ID: workspaceID.String()},
		AAD:   secretAAD(id),
	}
}

func secretAAD(id ids.UUID) []byte {
	return []byte("identity_secrets.secret_encrypted:" + id.String())
}

/*
sealSecret seals a shared secret into its slot.
*/
func sealSecret(ctx context.Context, keys *encryption.Keys, slot encryption.Slot, secret string,
) (encryption.Sealed, error) {
	sealed, err := encryption.Seal(ctx, keys, slot, secret)
	if err != nil {
		return nil, fmt.Errorf("seal identity secret: %w", err)
	}
	return sealed, nil
}

/*
parsedKey is an uploaded public key: what is stored and its fingerprint.
*/
type parsedKey struct {
	stored      json.RawMessage
	fingerprint string
}

/*
parsePublicKey checks an uploaded PEM public key against the algorithm
and returns what to store.
*/
func parsePublicKey(algorithm, pemText string) (parsedKey, error) {
	public, err := jwt.ParsePublicKeyPEM([]byte(pemText))
	if errors.Is(err, jwt.ErrPrivateKey) {
		return parsedKey{}, fault.New(fault.Invalid, "private_key_uploaded",
			"That is a private key. Upload the public key; your private key never leaves your server.").
			OnField("public_key")
	}
	if err != nil {
		return parsedKey{}, invalidKey()
	}
	if _, err := jwt.NewKey("", jwt.Algorithm(algorithm), public); err != nil {
		if errors.Is(err, jwt.ErrWeakKey) {
			return parsedKey{}, fault.New(fault.Invalid, "public_key_weak",
				"RSA keys must be at least 2048 bits.").OnField("public_key")
		}
		return parsedKey{}, invalidKey()
	}
	clean := strings.TrimSpace(pemText)
	stored, _ := json.Marshal(publicKey{PEM: clean})
	return parsedKey{stored: stored, fingerprint: hashing.SHA256Hex([]byte(clean))[:16]}, nil
}

func invalidKey() error {
	return fault.New(fault.Invalid, "public_key_invalid",
		"The public key must be PEM text matching the algorithm: RSA for RS256, Ed25519 for EdDSA.").
		OnField("public_key")
}

/*
verificationKey builds the key a secret verifies tokens with, named by
its public id.
*/
func verificationKey(ctx context.Context, keys *encryption.Keys, s models.IdentitySecret) (jwt.Key, error) {
	kid := models.PublicID(models.PrefixIdentitySecret, s.ID)
	if s.Algorithm == AlgorithmHS256 {
		secret, err := encryption.Open[string](ctx, keys, s.SecretEncrypted, secretAAD(s.ID))
		if err != nil {
			return jwt.Key{}, fmt.Errorf("open identity secret %s: %w", kid, err)
		}
		return jwt.NewKey(kid, jwt.HS256, []byte(secret))
	}
	var stored publicKey
	if err := json.Unmarshal(s.PublicKeyJwk, &stored); err != nil {
		return jwt.Key{}, fmt.Errorf("read public key %s: %w", kid, err)
	}
	public, err := jwt.ParsePublicKeyPEM([]byte(stored.PEM))
	if err != nil {
		return jwt.Key{}, fmt.Errorf("parse public key %s: %w", kid, err)
	}
	return jwt.NewKey(kid, jwt.Algorithm(s.Algorithm), public)
}
