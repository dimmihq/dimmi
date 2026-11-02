/*
|--------------------------------------------------------------------------
| Key Sets
|--------------------------------------------------------------------------
|
| Public keys come either as PEM, uploaded by whoever signs the tokens,
| or as a JWKS document published by an identity provider. A PEM holding
| a private key is refused outright, because the private half was never
| meant to leave its owner. A JWKS is fetched by the caller's own HTTP
| client, so outbound requests stay behind the application's guards, and
| it is cached; a token naming a key the cache does not know triggers
| one early refetch, which is how providers roll keys.
|
*/

package jwt

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	jose "github.com/go-jose/go-jose/v4"

	"github.com/dimmihq/dimmi/framework/clock"
)

/*
ParsePublicKeyPEM reads a PKIX public key from PEM. A private key gets
ErrPrivateKey, anything else ErrUnsupportedKey.
*/
func ParsePublicKeyPEM(data []byte) (crypto.PublicKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, ErrUnsupportedKey
	}

	if strings.Contains(block.Type, "PRIVATE KEY") {
		return nil, ErrPrivateKey
	}

	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnsupportedKey, err)
	}

	return key, nil
}

/*
ParseJWKS reads the signing keys of a JWKS document. Keys for other uses,
symmetric keys and keys of unsupported types or strength are skipped.
A key without alg is pinned to the one algorithm its type allows.
*/
func ParseJWKS(data []byte) ([]Key, error) {
	var set jose.JSONWebKeySet
	if err := json.Unmarshal(data, &set); err != nil {
		return nil, fmt.Errorf("parse jwks: %w", err)
	}

	keys := make([]Key, 0, len(set.Keys))

	for _, jwk := range set.Keys {
		if jwk.Use != "" && jwk.Use != "sig" {
			continue
		}

		public := jwk.Public()
		alg := Algorithm(jwk.Algorithm)

		if alg == "" {
			alg = inferAlgorithm(public.Key)
		}

		if key, err := NewKey(jwk.KeyID, alg, public.Key); err == nil && alg != HS256 {
			keys = append(keys, key)
		}
	}

	return keys, nil
}

func inferAlgorithm(public any) Algorithm {
	switch key := public.(type) {
	case *rsa.PublicKey:
		return RS256
	case *ecdsa.PublicKey:
		if key.Curve == elliptic.P256() {
			return ES256
		}
	case ed25519.PublicKey:
		return EdDSA
	}

	return ""
}

/*
Fetch returns the body at the URL. The caller supplies it, usually from
its outbound HTTP client.
*/
type Fetch func(ctx context.Context, url string) ([]byte, error)

/*
JWKSOptions configure a JWKS cache. TTL is how long fetched keys are
used before refetching, ten minutes by default. MinRefresh is the least
time between two fetches caused by unknown keys, one minute by default,
so tokens naming made-up keys cannot make the cache hammer the provider.
*/
type JWKSOptions struct {
	TTL        time.Duration
	MinRefresh time.Duration
	Clock      clock.Clock
}

/*
JWKS caches the keys published at one URL.
*/
type JWKS struct {
	url   string
	fetch Fetch
	opts  JWKSOptions

	mu        sync.Mutex
	keys      []Key
	fetchedAt time.Time
}

/*
NewJWKS returns a cache for the keys at the URL. Nothing is fetched
until keys are first needed.
*/
func NewJWKS(url string, fetch Fetch, opts JWKSOptions) *JWKS {
	if opts.TTL <= 0 {
		opts.TTL = 10 * time.Minute
	}

	if opts.MinRefresh <= 0 {
		opts.MinRefresh = time.Minute
	}

	if opts.Clock == nil {
		opts.Clock = clock.System()
	}

	return &JWKS{url: url, fetch: fetch, opts: opts}
}

/*
Keys returns the cached keys, fetching them when absent or older than
the TTL.
*/
func (s *JWKS) Keys(ctx context.Context) ([]Key, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.keys != nil && s.opts.Clock.Since(s.fetchedAt) < s.opts.TTL {
		return s.keys, nil
	}

	return s.refresh(ctx)
}

/*
Verify verifies the token against the published keys. When the token
names a key the cache lacks, the keys are fetched again once, unless
they were fetched within MinRefresh.
*/
func (s *JWKS) Verify(ctx context.Context, token string, opts Options) (Claims, error) {
	keys, err := s.Keys(ctx)
	if err != nil {
		return Claims{}, err
	}

	claims, err := Verify(token, keys, opts)
	if !errors.Is(err, ErrUnknownKey) {
		return claims, err
	}

	keys, err = s.refreshStale(ctx)
	if err != nil {
		return Claims{}, err
	}

	return Verify(token, keys, opts)
}

func (s *JWKS) refreshStale(ctx context.Context) ([]Key, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.opts.Clock.Since(s.fetchedAt) < s.opts.MinRefresh {
		return s.keys, nil
	}

	return s.refresh(ctx)
}

func (s *JWKS) refresh(ctx context.Context) ([]Key, error) {
	body, err := s.fetch(ctx, s.url)
	if err != nil {
		return nil, fmt.Errorf("fetch jwks %s: %w", s.url, err)
	}

	keys, err := ParseJWKS(body)
	if err != nil {
		return nil, err
	}

	s.keys, s.fetchedAt = keys, s.opts.Clock.Now()

	return keys, nil
}
