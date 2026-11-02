/*
|--------------------------------------------------------------------------
| Token Signing
|--------------------------------------------------------------------------
|
| Signing produces the compact form with typ "JWT" and the key's ID in
| kid, so a verifier holding several keys finds the right one. Times are
| written as whole seconds, as every JWT library expects.
|
*/

package jwt

import (
	"fmt"
	"maps"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	josejwt "github.com/go-jose/go-jose/v4/jwt"
)

/*
Sign returns the claims as a compact JWS signed with the key, which must
hold private material for asymmetric algorithms. Registered claims win
over Extra entries of the same name.
*/
func Sign(claims Claims, key Key) (string, error) {
	opts := (&jose.SignerOptions{}).WithType("JWT")
	if key.ID != "" {
		opts = opts.WithHeader(jose.HeaderKey("kid"), key.ID)
	}

	signer, err := jose.NewSigner(jose.SigningKey{
		Algorithm: jose.SignatureAlgorithm(key.Algorithm),
		Key:       key.material,
	}, opts)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrUnsupportedKey, err)
	}

	token, err := josejwt.Signed(signer).Claims(payloadOf(claims)).Serialize()
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}

	return token, nil
}

func payloadOf(claims Claims) map[string]any {
	payload := maps.Clone(claims.Extra)
	if payload == nil {
		payload = map[string]any{}
	}

	setString(payload, "iss", claims.Issuer)
	setString(payload, "sub", claims.Subject)
	setString(payload, "jti", claims.ID)
	setTime(payload, "exp", claims.Expiry)
	setTime(payload, "nbf", claims.NotBefore)
	setTime(payload, "iat", claims.IssuedAt)

	if len(claims.Audience) > 0 {
		payload["aud"] = josejwt.Audience(claims.Audience)
	}

	return payload
}

func setString(payload map[string]any, name, value string) {
	if value != "" {
		payload[name] = value
	}
}

func setTime(payload map[string]any, name string, value time.Time) {
	if !value.IsZero() {
		payload[name] = josejwt.NewNumericDate(value)
	}
}
