/*
|--------------------------------------------------------------------------
| Personal Access Tokens
|--------------------------------------------------------------------------
|
| A token is the application's prefix, 32 random letters and digits, and
| a six character checksum of both. The prefix lets people and secret
| scanners recognize a leaked token at a glance; the checksum lets the
| guard turn away a mistyped or made-up token without touching the
| database. Only a SHA-256 digest of the whole token is stored, which is
| enough for a value this random: there is nothing to guess from it.
|
| Recording every use would write on every request, so the last use is
| written at most once per touch interval.
|
*/

package token

import (
	"context"
	"errors"
	"fmt"
	"hash/crc32"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/auth"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/support/str"
)

/*
The lengths of a token's parts after the prefix, and of the hint kept
to recognize it by after the prefix.
*/
const (
	secretLength   = 32
	checksumLength = 6
	hintLength     = 4
)

/*
base62 is the alphabet of the secret and the checksum.
*/
const base62 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

/*
ErrInvalidPrefix is returned by New for an empty prefix or one with
characters other than letters, digits and underscores.
*/
var ErrInvalidPrefix = errors.New("token: prefix must be letters, digits and underscores")

/*
The faults a token that does not check out answers with. A token that
is malformed or unknown is auth.ErrInvalidToken.
*/
var (
	ErrTokenExpired = fault.New(fault.Unauthenticated, "token_expired", "The access token has expired.")
	ErrTokenRevoked = fault.New(fault.Unauthenticated, "token_revoked", "The access token has been revoked.")
)

var prefixPattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

/*
Token is a personal access token as stored, without its secret.

OwnerID is who the token acts for and TenantID the tenant it is bound
to, or "" for none. Name is the owner's label and Hint the prefix and
first characters of the token, to recognize it by. Abilities limit what
it may do; auth.AllAbilities grants everything. A zero ExpiresAt never
expires. LastUsedAt and LastUsedIP record its last use, and a non-zero
RevokedAt means it no longer works.
*/
type Token struct {
	ID         ids.UUID
	OwnerID    ids.UUID
	TenantID   string
	Name       string
	Hint       string
	Abilities  []string
	ExpiresAt  time.Time
	LastUsedAt time.Time
	LastUsedIP string
	RevokedAt  time.Time
	CreatedAt  time.Time
}

/*
Options configure the tokens.

Prefix starts every token, such as "app_live_". TouchInterval is how
often a token's last use is written (default one minute).
*/
type Options struct {
	Prefix        string        `koanf:"prefix"`
	TouchInterval time.Duration `koanf:"touch_interval"`
}

/*
Tokens issues, checks and revokes personal access tokens.
*/
type Tokens struct {
	store Store
	clock clock.Clock
	opts  Options
}

/*
New returns tokens on the store, or ErrInvalidPrefix.
*/
func New(store Store, clk clock.Clock, opts Options) (*Tokens, error) {
	if !prefixPattern.MatchString(opts.Prefix) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidPrefix, opts.Prefix)
	}

	if opts.TouchInterval <= 0 {
		opts.TouchInterval = time.Minute
	}

	return &Tokens{store: store, clock: clk, opts: opts}, nil
}

/*
Create issues a token for the owner, tenant, name, abilities and expiry
set on t, and returns it as stored along with the plain token, which is
never available again.
*/
func (t *Tokens) Create(ctx context.Context, tok Token) (Token, string, error) {
	plain := t.generate()
	now := t.clock.Now()

	tok.ID = ids.NewV7At(now)
	tok.Hint = plain[:len(t.opts.Prefix)+hintLength]
	tok.Abilities = append([]string{}, tok.Abilities...)
	tok.LastUsedAt, tok.LastUsedIP, tok.RevokedAt = time.Time{}, "", time.Time{}
	tok.CreatedAt = now

	if err := t.store.Insert(ctx, tok, digest(plain)); err != nil {
		return Token{}, "", err
	}

	return tok, plain, nil
}

/*
Find returns the live token for the plain value. A malformed or unknown
token is auth.ErrInvalidToken; an expired or revoked one is
ErrTokenExpired or ErrTokenRevoked.
*/
func (t *Tokens) Find(ctx context.Context, plain string) (Token, error) {
	if !t.wellFormed(plain) {
		return Token{}, auth.ErrInvalidToken
	}

	tok, err := t.store.FindByHash(ctx, digest(plain))
	if errors.Is(err, ErrNotFound) {
		return Token{}, auth.ErrInvalidToken
	}

	if err != nil {
		return Token{}, err
	}

	if !tok.RevokedAt.IsZero() {
		return Token{}, ErrTokenRevoked
	}

	if !tok.ExpiresAt.IsZero() && !t.clock.Now().Before(tok.ExpiresAt) {
		return Token{}, ErrTokenExpired
	}

	return tok, nil
}

/*
Use records that the token was used from the address, unless its last
use was recorded less than the touch interval ago.
*/
func (t *Tokens) Use(ctx context.Context, tok Token, ip string) error {
	now := t.clock.Now()
	if !tok.LastUsedAt.IsZero() && now.Sub(tok.LastUsedAt) < t.opts.TouchInterval {
		return nil
	}

	return t.store.Touch(ctx, tok.ID, now, ip)
}

/*
Revoke ends the token at once, or returns ErrNotFound when there is no
live token with the id.
*/
func (t *Tokens) Revoke(ctx context.Context, id ids.UUID) error {
	return t.store.Revoke(ctx, id, t.clock.Now())
}

/*
ForOwner returns the owner's tokens, revoked ones included, newest
first.
*/
func (t *Tokens) ForOwner(ctx context.Context, ownerID ids.UUID) ([]Token, error) {
	return t.store.ForOwner(ctx, ownerID)
}

/*
Resolver returns the bearer token resolver for these tokens. It answers
for tokens with the prefix only, records their use, and returns an
auth.Identity for the owner whose Guard is via, bound to the token's
tenant and carrying its abilities.
*/
func (t *Tokens) Resolver(via string) auth.TokenResolver {
	return auth.TokenResolverFunc(func(r *http.Request, plain string) (auth.Principal, error) {
		if !strings.HasPrefix(plain, t.opts.Prefix) {
			return nil, auth.ErrNoCredentials
		}

		tok, err := t.Find(r.Context(), plain)
		if err != nil {
			return nil, err
		}

		if err := t.Use(r.Context(), tok, http.ClientIP(r)); err != nil {
			return nil, err
		}

		return &auth.Identity{
			User:       tok.OwnerID,
			Via:        via,
			Credential: tok.ID,
			TenantID:   tok.TenantID,
			Granted:    slices.Clone(tok.Abilities),
		}, nil
	})
}

/*
generate returns a new plain token: the prefix, the random secret and
the checksum of both.
*/
func (t *Tokens) generate() string {
	body := t.opts.Prefix + str.Random(secretLength)

	return body + checksum(body)
}

/*
wellFormed reports whether the plain token has the prefix, the right
length, only base62 characters after the prefix and a matching
checksum.
*/
func (t *Tokens) wellFormed(plain string) bool {
	rest, ok := strings.CutPrefix(plain, t.opts.Prefix)
	if !ok || len(rest) != secretLength+checksumLength {
		return false
	}

	for _, c := range rest {
		if !strings.ContainsRune(base62, c) {
			return false
		}
	}

	split := len(plain) - checksumLength

	return hashing.Equal([]byte(checksum(plain[:split])), []byte(plain[split:]))
}

/*
checksum returns the CRC32 of the value in six base62 characters.
*/
func checksum(value string) string {
	sum := crc32.ChecksumIEEE([]byte(value))
	out := make([]byte, checksumLength)

	for i := checksumLength - 1; i >= 0; i-- {
		out[i] = base62[sum%62]
		sum /= 62
	}

	return string(out)
}

func digest(plain string) []byte {
	return hashing.SHA256([]byte(plain))
}
