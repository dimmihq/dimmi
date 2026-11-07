/*
|--------------------------------------------------------------------------
| Token Mistake Detector
|--------------------------------------------------------------------------
|
| Most failed tokens fail for one of a dozen reasons a developer would
| never guess from "signature does not match": the secret was base64
| decoded, read from a file with its newline, pasted with its quotes;
| exp is in milliseconds; the user id went in user_id. Once a token has
| failed, these checks try each known slip and name the one that fits.
| A match only explains: the token stays refused, whatever variant of a
| secret would have signed it. The work is bounded, a handful of HMACs
| on the failure path only.
|
*/

package identity

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Mistake is a recognized slip and the fix for it.
*/
type Mistake struct {
	Code string
	Hint string
}

/*
mistakes holds the hint of every slip the detector names.
*/
func mistakes() map[string]string {
	return map[string]string{
		"bearer_prefix":    "Send the token alone, without 'Bearer '.",
		"exp_milliseconds": "exp is in milliseconds. Use seconds: Math.floor(Date.now() / 1000) + 600.",
		"exp_iso_string":   "exp is a date string. Use seconds since the epoch, a number.",
		"user_id_alias":    "Put your user's id in sub, not user_id, userId or id.",
		"company_flat":     "Nest company fields: company: { id, name }, not company_id or company_name.",
		"secret_base64_decoded": "Your secret was base64-decoded before signing. Use the full dmi_idsec_ " +
			"string as is.",
		"secret_trailing_newline": "Your secret has a trailing newline, often from a file or echo. Trim it.",
		"secret_quoted":           "Your secret still has its quotes from the .env file. Remove them.",
		"secret_prefix_dropped":   "Your secret is missing its dmi_idsec_ start. Use the whole string.",
		"public_key_as_secret": "The token is HS256, signed with a public key's text. Sign RS256 or EdDSA with " +
			"your private key and set kid instead.",
		"other_workspace_key": "This key belongs to a different workspace. Use a secret from this workspace's " +
			"Settings > Identity verification.",
		"email_not_flagged": "email is present without email_verified, so it is not trusted to merge " +
			"conversations. Send email_verified: true when your app verified it.",
	}
}

func mistake(code string) *Mistake {
	return &Mistake{Code: code, Hint: mistakes()[code]}
}

/*
failed is a token that was refused: as sent, as read, and the code it
was refused with.
*/
type failed struct {
	token string
	read  payload
	code  string
}

/*
diagnose names the slip behind a failed token, nil when none fits.
*/
func (c *Checker) diagnose(ctx context.Context, ws ids.UUID, f failed) *Mistake {
	if strings.HasPrefix(strings.TrimSpace(f.token), "Bearer ") {
		return mistake("bearer_prefix")
	}
	if m := claimMistake(f.read); m != nil {
		return m
	}
	switch f.code {
	case "token_signature_invalid":
		return c.secretMistake(ctx, ws, f.token, f.read)
	case "token_kid_unknown":
		return c.foreignKey(ctx, f.read.header.KeyID)
	}
	return nil
}

/*
claimMistake recognizes claims written the wrong way.
*/
func claimMistake(p payload) *Mistake {
	switch exp := p.claims["exp"].(type) {
	case json.Number:
		if f, err := exp.Float64(); err == nil && f > 1e11 {
			return mistake("exp_milliseconds")
		}
	case string:
		return mistake("exp_iso_string")
	}
	if _, ok := p.claims["sub"]; !ok {
		for _, alias := range []string{"user_id", "userId", "id"} {
			if _, found := p.claims[alias]; found {
				return mistake("user_id_alias")
			}
		}
	}
	if _, ok := p.claims["company"]; !ok {
		_, id := p.claims["company_id"]
		_, name := p.claims["company_name"]
		if id || name {
			return mistake("company_flat")
		}
	}
	return nil
}

/*
secretMistake tries each usable shared secret in the forms developers
mangle it into, and public keys' text as an HMAC key.
*/
func (c *Checker) secretMistake(ctx context.Context, ws ids.UUID, token string, p payload) *Mistake {
	if p.header.Algorithm != AlgorithmHS256 {
		return nil
	}
	signed, signature, ok := splitSigned(token)
	if !ok {
		return nil
	}
	secrets, err := c.Verifier.secrets(ctx, ws)
	if err != nil {
		return nil
	}
	tried := 0
	for _, s := range secrets {
		if !c.Verifier.usable(s) || tried == maxTried {
			continue
		}
		tried++
		if m := c.variantOf(ctx, s, signed, signature); m != nil {
			return m
		}
	}
	return nil
}

/*
variantOf checks one secret's mangled forms against the signature.
*/
func (c *Checker) variantOf(ctx context.Context, s models.IdentitySecret, signed, signature []byte) *Mistake {
	if s.PublicKeyJwk != nil {
		var stored publicKey
		if json.Unmarshal(s.PublicKeyJwk, &stored) == nil && signs([]byte(stored.PEM), signed, signature) {
			return mistake("public_key_as_secret")
		}
		return nil
	}
	if s.SecretEncrypted == nil {
		return nil
	}
	secret, err := encryption.Open[string](ctx, c.Verifier.Keys, s.SecretEncrypted, secretAAD(s.ID))
	if err != nil {
		return nil
	}
	for code, variant := range variants(secret) {
		if signs(variant, signed, signature) {
			return mistake(code)
		}
	}
	return nil
}

/*
variants are the forms a secret is commonly mangled into.
*/
func variants(secret string) map[string][]byte {
	out := map[string][]byte{
		"secret_trailing_newline": []byte(secret + "\n"),
		"secret_quoted":           []byte(`"` + secret + `"`),
		"secret_prefix_dropped":   []byte(strings.TrimPrefix(secret, MaterialPrefix)),
	}
	body := strings.TrimPrefix(secret, MaterialPrefix)
	for _, enc := range []*base64.Encoding{base64.RawURLEncoding, base64.RawStdEncoding} {
		for _, candidate := range []string{secret, body} {
			if raw, err := enc.DecodeString(candidate); err == nil {
				out["secret_base64_decoded"] = raw
			}
		}
	}
	return out
}

/*
foreignKey reports a kid that is a key of another workspace.
*/
func (c *Checker) foreignKey(ctx context.Context, kid string) *Mistake {
	id, err := c.IDs.Decode(string(models.PrefixIdentitySecret), kid)
	if err != nil {
		return nil
	}
	found, err := models.New(c.Verifier.DB).IdentitySecretExists(ctx, id)
	if err != nil || !found {
		return nil
	}
	return mistake("other_workspace_key")
}

/*
splitSigned returns the signed part of a token and its signature bytes.
*/
func splitSigned(token string) ([]byte, []byte, bool) {
	token = strings.TrimSpace(token)
	i := strings.LastIndexByte(token, '.')
	if i < 0 {
		return nil, nil, false
	}
	signature, err := base64.RawURLEncoding.DecodeString(token[i+1:])
	if err != nil {
		return nil, nil, false
	}
	return []byte(token[:i]), signature, true
}

/*
signs reports whether HMAC-SHA256 with the key gives the signature.
*/
func signs(key, signed, signature []byte) bool {
	return hashing.Equal(hashing.HMAC(key, signed), signature)
}
