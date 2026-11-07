/*
|--------------------------------------------------------------------------
| Identity Token Failures
|--------------------------------------------------------------------------
|
| A developer wiring sign-in reads these in the browser console, so each
| says what is wrong in plain words and carries a hint naming the fix.
| None ever repeats the token or the secret. All answer 401, and none has
| any effect: the visitor carries on as they were.
|
*/

package identity

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/jwt"
)

/*
tokenError is a refused token: a code, a message and a hint.
*/
func tokenError(code, message, hint string) *fault.Error {
	err := fault.New(fault.Unauthenticated, code, message)
	if hint == "" {
		return err
	}
	return err.With("hint", hint)
}

/*
Failures that read the same every time.
*/
var (
	ErrNotConfigured = tokenError("identity_not_configured",
		"No identity secret exists for this workspace.",
		"Create one in Settings > Identity verification, then sign tokens with it on your server.")
	ErrTokenMalformed = tokenError("token_malformed",
		"Token is not a JWT (needs three base64url parts separated by dots).",
		"Pass the token string, not the JSON payload or 'Bearer ...'.")
	ErrKidRequired = tokenError("token_kid_required",
		"RS256 and EdDSA tokens need a kid header.",
		"Set kid to the key id shown in Settings > Identity verification.")
	ErrSignatureInvalid = tokenError("token_signature_invalid",
		"Signature does not match.",
		"Use the full dmi_idsec_ string as the key (no base64 decoding, no trailing newline), "+
			"from this workspace.")
	ErrExpMissing = tokenError("token_exp_missing",
		"Token has no exp claim.",
		"Add exp in seconds since the epoch, such as now + 600.")
	ErrSubMissing = tokenError("token_sub_missing",
		"Token has no sub claim.",
		"Set sub (not user_id) to your user's id.")
	ErrSubInvalid = tokenError("token_sub_invalid",
		"sub must be a string of 1 to 255 characters.",
		"Send your user's id as a string, such as \"42\".")
	ErrAudience = tokenError("token_audience_mismatch",
		"The token's aud is not for this messenger.",
		"Use 'dimmi', the messenger's app id or the workspace id, or leave aud out.")
	ErrReplayed = tokenError("token_replayed",
		"This token's nonce was already used.",
		"Generate a new nonce for every token, or leave nonce out.")
	ErrJWKSUnavailable = tokenError("token_jwks_unavailable",
		"Your auth provider's keys could not be fetched for over a day.",
		"Check the key set address in Settings > Identity verification answers.")
	ErrIssuer = tokenError("token_issuer_mismatch",
		"The token's iss is not the issuer this key set expects.",
		"Set the issuer in Settings > Identity verification to your provider's iss.")
	ErrNonceUnavailable = fault.New(fault.Unavailable, "identity_unavailable",
		"Tokens with a nonce cannot be checked right now; try again shortly.")
)

/*
tooLarge refuses a token over the size limit.
*/
func tooLarge(size, limit int) *fault.Error {
	return tokenError("token_too_large",
		fmt.Sprintf("Token is %d KB; the limit is %d KB.", int(math.Ceil(float64(size)/1024)), limit/1024),
		"Keep tokens small: update large attributes through the REST API instead.")
}

func algNotAllowed(alg string) *fault.Error {
	return tokenError("token_alg_not_allowed", fmt.Sprintf("Algorithm %q is not accepted.", alg),
		"Use HS256, RS256, ES256 or EdDSA.")
}

func kidUnknown(kid string) *fault.Error {
	return tokenError("token_kid_unknown", fmt.Sprintf("No key %s in this workspace.", kid),
		"Copy the key id from Settings, or remove the kid header to try every HS256 secret.")
}

func secretRevoked(kid string, at *time.Time) *fault.Error {
	when := ""
	if at != nil {
		when = " on " + at.UTC().Format("2 January 2006")
	}
	return tokenError("token_secret_revoked", fmt.Sprintf("Key %s was revoked%s.", kid, when),
		"Sign with an active secret.")
}

func algMismatch(alg, kid, keyAlg string) *fault.Error {
	return tokenError("token_alg_mismatch", fmt.Sprintf("Token uses %s but key %s is %s.", alg, kid, keyAlg),
		"Sign with the algorithm the key was made for.")
}

/*
fromJWT turns a verification failure into what the developer reads.
*/
func fromJWT(err error, p payload, now time.Time) error {
	switch {
	case errors.Is(err, jwt.ErrSignatureInvalid):
		return ErrSignatureInvalid
	case errors.Is(err, jwt.ErrMissingExpiry):
		return ErrExpMissing
	case errors.Is(err, jwt.ErrExpired):
		return tokenError("token_expired", "The identity token has expired.",
			fmt.Sprintf("It expired %s ago. Mint a token per request from your identity endpoint; never cache "+
				"it or put it in your HTML.", ago(now.Sub(p.exp()))))
	case errors.Is(err, jwt.ErrNotYetValid):
		return tokenError("token_not_yet_valid", "Token is not valid yet; your server clock looks ahead.",
			"Sync your server's clock (NTP).")
	case errors.Is(err, jwt.ErrLifetimeTooLong):
		return tokenError("token_lifetime_too_long",
			fmt.Sprintf("Token lives %s; the most is 7 days.", ago(p.lifetime(now))),
			"Use 10 minutes, in seconds not milliseconds; the messenger asks your server again when needed.")
	}
	return ErrTokenMalformed
}

/*
ago writes a duration the way a person would say it.
*/
func ago(d time.Duration) string {
	switch {
	case d < 2*time.Minute:
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	case d < 2*time.Hour:
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%d hours", int(d.Hours()))
	}
	return fmt.Sprintf("%d days", int(d.Hours()/24))
}
