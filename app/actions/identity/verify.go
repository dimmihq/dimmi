/*
|--------------------------------------------------------------------------
| Verify Identity Token
|--------------------------------------------------------------------------
|
| A host app's server signs a short-lived token naming its signed-in user,
| or hands over the one its auth provider issued, and only a token that
| passes every check here identifies anyone. The key comes from the kid
| header: a dimmi key id names one secret, any other kid is looked up in
| the provider key sets the workspace trusts, and for HS256 without one
| each usable shared secret is tried. The workspace comes from the
| messenger, never from the token, so a token signed for another
| workspace fails. A token with a nonce works once. Verifying writes
| nothing but a throttled note of when the secret was last used; a dry
| check, the token checker's, writes nothing at all.
|
*/

package identity

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/jwt"

	"github.com/dimmihq/dimmi/app/models"
)

/*
maxTried bounds how many shared secrets a token without a kid, or key
sets a provider's token, is tried against.
*/
const maxTried = 5

/*
algorithms are the signing algorithms a token may use at all.
*/
func algorithms() []string {
	return []string{AlgorithmHS256, AlgorithmRS256, AlgorithmES256, AlgorithmEdDSA}
}

/*
Verifier checks identity tokens. Cache remembers used nonces and
KeySets fetches providers' key sets. Leeway forgives clock skew and
MaxLifetime bounds how long a token may live. Passes redeems the help
center's passes, nil where none are accepted.
*/
type Verifier struct {
	DB          *database.DB
	Clock       clock.Clock
	Keys        *encryption.Keys
	Cache       *cache.Store
	KeySets     *KeySets
	Leeway      time.Duration
	MaxLifetime time.Duration
	Passes      *Passes
}

/*
Token is a token to check, sent to the messenger with its public app id,
inside its workspace.
*/
type Token struct {
	WorkspaceID ids.UUID
	AppID       string
	Value       string
}

/*
Verified is a token that passed: the secret that signed it, the
customer's id and what it says about them. A help center pass sets
Contact instead, the workspace's own contact it signs in, and leaves the
rest empty.
*/
type Verified struct {
	SecretID ids.UUID
	Profile  Profile
	Contact  *ids.UUID
}

/*
Verify checks the token and returns who it names. Errors are faults
with a code and a hint, safe to show the developer. A help center pass
is redeemed instead of checked.
*/
func (v *Verifier) Verify(ctx context.Context, t Token) (Verified, error) {
	if IsPass(t.Value) && v.Passes != nil {
		contact, err := v.Passes.Redeem(ctx, t.WorkspaceID, t.Value)
		return Verified{Contact: &contact}, err
	}
	passed, err := v.run(ctx, t)
	if err != nil {
		return Verified{}, err
	}
	if err := v.spendNonce(ctx, t.WorkspaceID, passed.claims); err != nil {
		return Verified{}, err
	}
	v.touch(ctx, passed.secret)
	return verified(passed), nil
}

/*
Check verifies the token like Verify but spends no nonce and notes
nothing, so a developer can test a token and still use it.
*/
func (v *Verifier) Check(ctx context.Context, t Token) (Verified, error) {
	passed, err := v.run(ctx, t)
	if err != nil {
		return Verified{}, err
	}
	return verified(passed), nil
}

/*
passed is a token whose signature and claims checked out: the secret
that verified it, its claims and its subject.
*/
type passed struct {
	secret models.IdentitySecret
	claims jwt.Claims
	sub    string
}

/*
run checks everything but the nonce.
*/
func (v *Verifier) run(ctx context.Context, t Token) (passed, error) {
	secrets, err := v.secrets(ctx, t.WorkspaceID)
	if err != nil {
		return passed{}, err
	}
	p, err := readToken(t.Value)
	if err != nil {
		return passed{}, err
	}
	sub, err := subject(p)
	if err != nil {
		return passed{}, err
	}
	secret, claims, err := v.pickAndVerify(ctx, t.Value, p, secrets)
	if err != nil {
		return passed{}, err
	}
	if err := audienceOK(claims.Audience, t, secret); err != nil {
		return passed{}, err
	}
	return passed{secret: secret, claims: claims, sub: sub}, nil
}

func verified(p passed) Verified {
	profile := ReadProfile(p.claims.Extra)
	profile.ExternalID = p.sub
	return Verified{SecretID: p.secret.ID, Profile: profile}
}

/*
secrets loads every secret of the workspace, revoked ones included so a
token signed with one is told so.
*/
func (v *Verifier) secrets(ctx context.Context, workspaceID ids.UUID) ([]models.IdentitySecret, error) {
	var out []models.IdentitySecret
	err := v.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = models.New(tx).ListIdentitySecrets(ctx, workspaceID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("load identity secrets: %w", err)
	}
	if !slices.ContainsFunc(out, v.usable) {
		return nil, ErrNotConfigured
	}
	return out, nil
}

/*
usable reports whether the secret still verifies tokens: active, or
retiring and not yet past its date.
*/
func (v *Verifier) usable(s models.IdentitySecret) bool {
	switch s.Status {
	case "active":
		return true
	case "retiring":
		return s.RetiresAt != nil && v.Clock.Now().Before(*s.RetiresAt)
	}
	return false
}

/*
pickAndVerify finds the key the token names and verifies it.
*/
func (v *Verifier) pickAndVerify(ctx context.Context, token string, p payload, secrets []models.IdentitySecret,
) (models.IdentitySecret, jwt.Claims, error) {
	if !slices.Contains(algorithms(), p.header.Algorithm) {
		return models.IdentitySecret{}, jwt.Claims{}, algNotAllowed(p.header.Algorithm)
	}
	if p.header.KeyID != "" && !strings.HasPrefix(p.header.KeyID, string(models.PrefixIdentitySecret)+"_") {
		return v.checkProvider(ctx, token, p, secrets)
	}
	if p.header.KeyID != "" {
		return v.checkNamed(ctx, token, p, secrets)
	}
	if p.header.Algorithm != AlgorithmHS256 {
		return models.IdentitySecret{}, jwt.Claims{}, ErrKidRequired
	}
	return v.checkEach(ctx, token, p, secrets)
}

/*
checkNamed verifies the token with the secret its kid names.
*/
func (v *Verifier) checkNamed(ctx context.Context, token string, p payload, secrets []models.IdentitySecret,
) (models.IdentitySecret, jwt.Claims, error) {
	kid := p.header.KeyID
	i := slices.IndexFunc(secrets, func(s models.IdentitySecret) bool {
		return s.JwksUrl == nil && models.PublicID(models.PrefixIdentitySecret, s.ID) == kid
	})
	if i < 0 {
		return models.IdentitySecret{}, jwt.Claims{}, kidUnknown(kid)
	}
	s := secrets[i]
	if !v.usable(s) {
		return s, jwt.Claims{}, secretRevoked(kid, revokedAt(s))
	}
	if s.Algorithm != p.header.Algorithm {
		return s, jwt.Claims{}, algMismatch(p.header.Algorithm, kid, s.Algorithm)
	}
	claims, err := v.verifyWith(ctx, token, p, s)
	return s, claims, err
}

/*
checkEach tries the token against every usable shared secret. A failure
other than the signature means that secret signed it, so it is the
answer.
*/
func (v *Verifier) checkEach(ctx context.Context, token string, p payload, secrets []models.IdentitySecret,
) (models.IdentitySecret, jwt.Claims, error) {
	tried := 0
	for _, s := range secrets {
		if s.Algorithm != AlgorithmHS256 || !v.usable(s) || tried == maxTried {
			continue
		}
		tried++
		claims, err := v.verifyWith(ctx, token, p, s)
		if !errors.Is(err, ErrSignatureInvalid) {
			return s, claims, err
		}
	}
	return models.IdentitySecret{}, jwt.Claims{}, ErrSignatureInvalid
}

/*
checkProvider looks the token's kid up in the provider key sets the
workspace trusts and verifies it with the key found, then checks the
issuer. A set that cannot be fetched is reported only when no other set
holds the key.
*/
func (v *Verifier) checkProvider(ctx context.Context, token string, p payload, secrets []models.IdentitySecret,
) (models.IdentitySecret, jwt.Claims, error) {
	kid, tried := p.header.KeyID, 0
	var unavailable error
	for _, s := range secrets {
		if s.JwksUrl == nil || !v.usable(s) || tried == maxTried || v.KeySets == nil {
			continue
		}
		tried++
		fresh, keys, err := v.KeySets.providerKeys(ctx, s, kid)
		if err != nil {
			unavailable = err
			continue
		}
		if !hasKey(keys, kid) {
			continue
		}
		if fresh.Algorithm != p.header.Algorithm {
			return fresh, jwt.Claims{}, algMismatch(p.header.Algorithm, kid, fresh.Algorithm)
		}
		claims, err := v.verifyKeys(token, p, keys, fresh.Algorithm)
		if err == nil && !sameIssuer(claims.Issuer, expectedIssuer(fresh)) {
			err = ErrIssuer
		}
		return fresh, claims, err
	}
	if unavailable != nil {
		return models.IdentitySecret{}, jwt.Claims{}, unavailable
	}
	return models.IdentitySecret{}, jwt.Claims{}, kidUnknown(kid)
}

/*
verifyWith verifies the token with one secret's key.
*/
func (v *Verifier) verifyWith(ctx context.Context, token string, p payload, s models.IdentitySecret,
) (jwt.Claims, error) {
	key, err := verificationKey(ctx, v.Keys, s)
	if err != nil {
		return jwt.Claims{}, err
	}
	return v.verifyKeys(token, p, []jwt.Key{key}, s.Algorithm)
}

/*
verifyKeys verifies the token against keys pinned to one algorithm.
*/
func (v *Verifier) verifyKeys(token string, p payload, keys []jwt.Key, algorithm string) (jwt.Claims, error) {
	claims, err := jwt.Verify(token, keys, jwt.Options{
		Algorithms: []jwt.Algorithm{jwt.Algorithm(algorithm)}, Leeway: v.Leeway,
		MaxLifetime: v.MaxLifetime, Clock: v.Clock,
	})
	if err != nil {
		return claims, fromJWT(err, p, v.Clock.Now())
	}
	return claims, nil
}

/*
audienceOK accepts a token without aud, or one naming dimmi, the
messenger, the workspace or the secret's own audience. A provider's
token is pinned by its issuer and usually carries the provider's
audience, so for a key set without an audience of its own any aud
passes.
*/
func audienceOK(aud []string, t Token, s models.IdentitySecret) error {
	if len(aud) == 0 || (s.JwksUrl != nil && deref(s.Audience) == "") {
		return nil
	}
	accepted := []string{"dimmi", t.AppID, models.PublicID(models.PrefixWorkspace, t.WorkspaceID)}
	if s.Audience != nil && *s.Audience != "" {
		accepted = append(accepted, *s.Audience)
	}
	for _, a := range aud {
		if slices.Contains(accepted, a) {
			return nil
		}
	}
	return ErrAudience
}

/*
subject returns the token's sub, trimmed, refusing a missing, numeric,
empty, overlong or control-character one. It is read before the
signature is checked, so a numeric sub is named as such instead of
failing as an unreadable token; nothing is trusted until the signature
passes.
*/
func subject(p payload) (string, error) {
	raw, ok := p.claims["sub"]
	if !ok || raw == nil {
		return "", ErrSubMissing
	}
	s, ok := raw.(string)
	if !ok {
		return "", ErrSubInvalid
	}
	s = strings.TrimSpace(s)
	if s == "" || len([]rune(s)) > 255 || strings.ContainsFunc(s, unicode.IsControl) {
		return "", ErrSubInvalid
	}
	return s, nil
}

/*
spendNonce makes a token with a nonce work only once, for as long as it
could still be accepted.
*/
func (v *Verifier) spendNonce(ctx context.Context, workspaceID ids.UUID, claims jwt.Claims) error {
	nonce, _ := claims.Extra["nonce"].(string)
	if nonce == "" {
		return nil
	}
	if len(nonce) > 128 {
		return ErrTokenMalformed
	}
	key := "identity:nonce:" + workspaceID.String() + ":" + hashing.SHA256Hex([]byte(nonce))
	ttl := claims.Expiry.Sub(v.Clock.Now()) + v.Leeway
	added, err := v.Cache.Add(ctx, key, []byte("1"), max(ttl, time.Second))
	if err != nil {
		return ErrNonceUnavailable
	}
	if !added {
		return ErrReplayed
	}
	return nil
}

/*
touch notes the secret was used, at most once a minute. A failure costs
only the note.
*/
func (v *Verifier) touch(ctx context.Context, s models.IdentitySecret) {
	now := v.Clock.Now()
	if s.LastUsedAt != nil && now.Sub(*s.LastUsedAt) < time.Minute {
		return
	}
	_ = v.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		return models.New(tx).TouchIdentitySecret(ctx, models.TouchIdentitySecretParams{
			Now: &now, WorkspaceID: s.WorkspaceID, ID: s.ID, StaleBefore: new(now.Add(-time.Minute)),
		})
	})
}

func revokedAt(s models.IdentitySecret) *time.Time {
	if s.RevokedAt != nil {
		return s.RevokedAt
	}
	return s.RetiresAt
}

/*
header is the part of a token's header that picks the key.
*/
type header struct {
	Algorithm string `json:"alg"`
	KeyID     string `json:"kid"`
}

/*
payload is a token read before verification, only to pick its key and
explain a failure; nothing in it is trusted.
*/
type payload struct {
	header header
	claims map[string]any
}

/*
readToken reads a token's header and claims, refusing anything that is
not a compact JWT under the size limit.
*/
func readToken(token string) (payload, error) {
	token = strings.TrimSpace(token)
	if len(token) > jwt.DefaultMaxSize {
		return payload{}, tooLarge(len(token), jwt.DefaultMaxSize)
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return payload{}, ErrTokenMalformed
	}
	p := payload{}
	if decodePart(parts[0], &p.header) != nil || decodePart(parts[1], &p.claims) != nil || p.claims == nil {
		return payload{}, ErrTokenMalformed
	}
	return p, nil
}

func decodePart(part string, into any) error {
	raw, err := base64.RawURLEncoding.DecodeString(part)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	return dec.Decode(into)
}

/*
exp is the token's expiry as written, zero when it has none.
*/
func (p payload) exp() time.Time {
	return p.time("exp")
}

/*
lifetime is how long the token claims to live, from iat or now.
*/
func (p payload) lifetime(now time.Time) time.Duration {
	start := p.time("iat")
	if start.IsZero() {
		start = now
	}
	return p.exp().Sub(start)
}

func (p payload) time(name string) time.Time {
	n, ok := p.claims[name].(json.Number)
	if !ok {
		return time.Time{}
	}
	f, err := n.Float64()
	if err != nil {
		return time.Time{}
	}
	return time.Unix(int64(f), 0)
}
