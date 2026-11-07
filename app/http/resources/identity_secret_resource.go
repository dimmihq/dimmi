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
token's kid header names it by.
*/
type IdentitySecret struct {
	ID          string     `json:"id"`
	Object      string     `json:"object"`
	Name        string     `json:"name"`
	Algorithm   string     `json:"algorithm"`
	Kid         string     `json:"kid"`
	Fingerprint string     `json:"fingerprint"`
	Status      string     `json:"status"`
	RetiresAt   *time.Time `json:"retires_at"`
	RevokedAt   *time.Time `json:"revoked_at"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

/*
NewIdentitySecret shapes a secret for the API.
*/
func NewIdentitySecret(s models.IdentitySecret) IdentitySecret {
	id := models.PublicID(models.PrefixIdentitySecret, s.ID)
	return IdentitySecret{
		ID: id, Object: "identity_secret", Name: s.Name, Algorithm: s.Algorithm, Kid: id,
		Fingerprint: s.Fingerprint, Status: s.Status, RetiresAt: s.RetiresAt, RevokedAt: s.RevokedAt,
		LastUsedAt: s.LastUsedAt, CreatedAt: s.CreatedAt,
	}
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
