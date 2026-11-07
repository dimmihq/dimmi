/*
|--------------------------------------------------------------------------
| Identity Secret Resource
|--------------------------------------------------------------------------
|
| A secret as the API shows it: never its material, only a fingerprint to
| tell secrets apart. The one exception is the answer that created a
| shared secret, which carries it once, with the names of the environment
| variables a developer's server reads it from.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/models"
)

/*
IdentitySecret is one secret without its material. Kid is the value a
token's kid header names it by, empty for a provider's key set, whose
tokens carry the provider's own key ids. Source says what it verifies
with: a shared secret, a public key or a key set, the last with its
address, issuer and when it was last fetched or why it failed.
*/
type IdentitySecret struct {
	ID            string     `json:"id"`
	Object        string     `json:"object"`
	Name          string     `json:"name"`
	Algorithm     string     `json:"algorithm"`
	Source        string     `json:"source"`
	Kid           string     `json:"kid"`
	JWKSURL       *string    `json:"jwks_url"`
	Issuer        *string    `json:"issuer"`
	Audience      *string    `json:"audience"`
	JWKSFetchedAt *time.Time `json:"jwks_fetched_at"`
	JWKSError     *string    `json:"jwks_error"`
	Fingerprint   string     `json:"fingerprint"`
	Status        string     `json:"status"`
	RetiresAt     *time.Time `json:"retires_at"`
	RevokedAt     *time.Time `json:"revoked_at"`
	LastUsedAt    *time.Time `json:"last_used_at"`
	CreatedAt     time.Time  `json:"created_at"`
}

/*
NewIdentitySecret shapes a secret for the API.
*/
func NewIdentitySecret(s models.IdentitySecret) IdentitySecret {
	id := models.PublicID(models.PrefixIdentitySecret, s.ID)
	out := IdentitySecret{
		ID: id, Object: "identity_secret", Name: s.Name, Algorithm: s.Algorithm, Source: "shared_secret", Kid: id,
		Audience: s.Audience, Fingerprint: s.Fingerprint, Status: s.Status, RetiresAt: s.RetiresAt,
		RevokedAt: s.RevokedAt, LastUsedAt: s.LastUsedAt, CreatedAt: s.CreatedAt,
	}
	switch {
	case s.JwksUrl != nil:
		out.Source, out.Kid, out.JWKSURL, out.Issuer = "jwks", "", s.JwksUrl, s.JwksIssuer
		out.JWKSFetchedAt, out.JWKSError = s.JwksFetchedAt, s.JwksError
	case s.PublicKeyJwk != nil:
		out.Source = "public_key"
	}
	return out
}

/*
NewIdentitySecrets shapes a list of secrets.
*/
func NewIdentitySecrets(rows []models.IdentitySecret) []IdentitySecret {
	out := make([]IdentitySecret, len(rows))
	for i, r := range rows {
		out[i] = NewIdentitySecret(r)
	}
	return out
}

/*
CreatedIdentitySecret is a new secret with its shared secret, empty for
a public key, shown only in this answer.
*/
type CreatedIdentitySecret struct {
	IdentitySecret
	Secret string `json:"secret,omitempty"`
}

/*
CreatedIdentitySecretMeta tells the developer the secret shows once and
where their server should read it from.
*/
type CreatedIdentitySecretMeta struct {
	SecretShownOnce bool   `json:"secret_shown_once"`
	EnvVar          string `json:"env_var"`
	KidEnvVar       string `json:"kid_env_var"`
}
