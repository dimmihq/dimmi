/*
|--------------------------------------------------------------------------
| Token Verification
|--------------------------------------------------------------------------
|
| Verification runs in a fixed order so the cheapest and most telling
| refusal comes first: format, then header rules and algorithm, then key
| and signature, then the claims. Every failure is one of the package
| errors, so the caller can tell the token's author what to fix.
|
*/

package jwt

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	josejwt "github.com/go-jose/go-jose/v4/jwt"

	"github.com/dimmihq/dimmi/framework/clock"
)

/*
header is the part of a JOSE header verification reads.
*/
type header struct {
	Algorithm Algorithm       `json:"alg"`
	KeyID     string          `json:"kid"`
	Type      *string         `json:"typ"`
	Critical  json.RawMessage `json:"crit"`
}

/*
Verify checks the token against the keys and options and returns its
claims. The key is chosen by the kid header, or is the only key when
the token names none.
*/
func Verify(token string, keys []Key, opts Options) (Claims, error) {
	opts = withDefaults(opts)

	token, parts, err := splitCompact(token, opts.MaxSize)
	if err != nil {
		return Claims{}, err
	}

	key, err := selectKey(parts.header, keys, opts)
	if err != nil {
		return Claims{}, err
	}

	var registered josejwt.Claims
	if err := json.Unmarshal(parts.payload, &registered); err != nil {
		return Claims{}, fmt.Errorf("%w: %w", ErrMalformed, err)
	}

	if err := verifySignature(token, key); err != nil {
		return Claims{}, err
	}

	if err := checkClaims(registered, opts); err != nil {
		return Claims{}, err
	}

	return toClaims(registered, parts.payload)
}

func withDefaults(opts Options) Options {
	if opts.Leeway == 0 {
		opts.Leeway = DefaultLeeway
	}

	if opts.MaxLifetime == 0 {
		opts.MaxLifetime = DefaultMaxLifetime
	}

	if opts.MaxSize == 0 {
		opts.MaxSize = DefaultMaxSize
	}

	if opts.Clock == nil {
		opts.Clock = clock.System()
	}

	return opts
}

func selectKey(raw []byte, keys []Key, opts Options) (Key, error) {
	var h header
	if err := json.Unmarshal(raw, &h); err != nil {
		return Key{}, fmt.Errorf("%w: %w", ErrMalformed, err)
	}

	if h.Critical != nil {
		return Key{}, fmt.Errorf("%w: crit header is not supported", ErrMalformed)
	}

	if !typeAccepted(h.Type, opts.Type) {
		return Key{}, fmt.Errorf("%w: unexpected typ", ErrMalformed)
	}

	if !supported(h.Algorithm) || !slices.Contains(opts.Algorithms, h.Algorithm) {
		return Key{}, fmt.Errorf("%w: %q", ErrAlgNotAllowed, h.Algorithm)
	}

	key, ok := findKey(keys, h.KeyID)
	if !ok {
		return Key{}, fmt.Errorf("%w: %q", ErrUnknownKey, h.KeyID)
	}

	if key.Algorithm != h.Algorithm {
		return Key{}, fmt.Errorf("%w: key %q is %s", ErrAlgMismatch, key.ID, key.Algorithm)
	}

	return key, nil
}

/*
typeAccepted applies the typ rule: the required type when one is set,
else absent or JWT. Media types compare without case and may carry the
"application/" prefix.
*/
func typeAccepted(got *string, want string) bool {
	if want == "" {
		want = "JWT"
		if got == nil {
			return true
		}
	}

	if got == nil {
		return false
	}

	typ := *got
	if strings.HasPrefix(strings.ToLower(typ), "application/") {
		typ = typ[len("application/"):]
	}

	return strings.EqualFold(typ, want)
}

func supported(alg Algorithm) bool {
	return alg == HS256 || alg == RS256 || alg == ES256 || alg == EdDSA
}

func findKey(keys []Key, id string) (Key, bool) {
	if id == "" {
		if len(keys) == 1 {
			return keys[0], true
		}

		return Key{}, false
	}

	i := slices.IndexFunc(keys, func(k Key) bool { return k.ID == id })
	if i < 0 {
		return Key{}, false
	}

	return keys[i], true
}

func verifySignature(token string, key Key) error {
	parsed, err := josejwt.ParseSigned(token, []jose.SignatureAlgorithm{jose.SignatureAlgorithm(key.Algorithm)})
	if err != nil {
		return fmt.Errorf("%w: %w", ErrMalformed, err)
	}

	if err := parsed.Claims(publicOf(key.material)); err != nil {
		return ErrSignatureInvalid
	}

	return nil
}

func checkClaims(registered josejwt.Claims, opts Options) error {
	if registered.Expiry == nil {
		return ErrMissingExpiry
	}

	now := opts.Clock.Now()
	expected := josejwt.Expected{Issuer: opts.Issuer, Time: now}

	if opts.Audience != "" {
		expected.AnyAudience = josejwt.Audience{opts.Audience}
	}

	if err := registered.ValidateWithLeeway(expected, opts.Leeway); err != nil {
		return mapValidationError(err)
	}

	start := now
	if registered.IssuedAt != nil {
		start = registered.IssuedAt.Time()
	}

	if registered.Expiry.Time().Sub(start) > opts.MaxLifetime {
		return fmt.Errorf("%w: exp is in seconds and at most %s ahead", ErrLifetimeTooLong, opts.MaxLifetime)
	}

	return nil
}

func mapValidationError(err error) error {
	switch {
	case errors.Is(err, josejwt.ErrExpired):
		return ErrExpired
	case errors.Is(err, josejwt.ErrNotValidYet), errors.Is(err, josejwt.ErrIssuedInTheFuture):
		return ErrNotYetValid
	case errors.Is(err, josejwt.ErrInvalidAudience):
		return ErrAudience
	case errors.Is(err, josejwt.ErrInvalidIssuer):
		return ErrIssuer
	}

	return fmt.Errorf("%w: %w", ErrMalformed, err)
}

func toClaims(registered josejwt.Claims, payload []byte) (Claims, error) {
	var extra map[string]any

	dec := json.NewDecoder(strings.NewReader(string(payload)))
	dec.UseNumber()

	if err := dec.Decode(&extra); err != nil {
		return Claims{}, fmt.Errorf("%w: %w", ErrMalformed, err)
	}

	for _, name := range []string{"iss", "sub", "aud", "jti", "exp", "nbf", "iat"} {
		delete(extra, name)
	}

	return Claims{
		Issuer:    registered.Issuer,
		Subject:   registered.Subject,
		Audience:  registered.Audience,
		ID:        registered.ID,
		Expiry:    timeOf(registered.Expiry),
		NotBefore: timeOf(registered.NotBefore),
		IssuedAt:  timeOf(registered.IssuedAt),
		Extra:     extra,
	}, nil
}

func timeOf(date *josejwt.NumericDate) time.Time {
	if date == nil {
		return time.Time{}
	}

	return date.Time()
}
