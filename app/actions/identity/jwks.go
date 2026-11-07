/*
|--------------------------------------------------------------------------
| Provider Key Sets
|--------------------------------------------------------------------------
|
| A host app that signs its users in with Clerk, Auth0, Supabase or any
| provider publishing a JWKS can hand the messenger the provider's own
| token: the secret points at the provider's key set instead of holding
| a key. The set is fetched only from the address the owner typed in the
| dashboard, never from anything a token says, and cached on the row so
| every replica verifies with the same copy. It is fetched again every
| six hours, and at most once every five minutes when a token names a
| key the copy lacks, which is how a provider's key rotation is picked
| up. A copy older than a day that cannot be refreshed stops verifying:
| a provider that cannot be reached must not keep revoked keys alive.
|
*/

package identity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/jwt"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Limits of fetching a key set: five seconds, 512 KB. A copy is refreshed
when a token names an unknown key at most every RetryUnknown, and stops
verifying once older than MaxStale with no fresher copy to be had.
RefreshEvery is how old a copy gets before the scheduled round fetches
it again.
*/
const (
	jwksTimeout  = 5 * time.Second
	jwksMaxBytes = 512 << 10
	RetryUnknown = 5 * time.Minute
	MaxStale     = 24 * time.Hour
	RefreshEvery = 6 * time.Hour
)

/*
Failures of fetching a provider's key set, shown to the owner who typed
its address.
*/
var (
	ErrJWKSURLInvalid = fault.New(fault.Invalid, "jwks_url_invalid",
		"The key set address must be an https URL, such as https://your-app.clerk.accounts.dev/.well-known/jwks.json.").
		OnField("jwks_url")
	ErrJWKSUnreachable = fault.New(fault.Invalid, "jwks_unreachable",
		"The key set address could not be reached or did not answer with a key set.").OnField("jwks_url")
	ErrJWKSInvalid = fault.New(fault.Invalid, "jwks_invalid",
		"The address answered, but with no signing key dimmi accepts: RSA of at least 2048 bits, P-256 or Ed25519.").
		OnField("jwks_url")
)

/*
KeySets fetches and caches providers' key sets.
*/
type KeySets struct {
	DB    *database.DB
	HTTP  *httpclient.Client
	Clock clock.Clock
}

/*
fetch reads the key set at the address and checks it holds at least one
signing key for the algorithm.
*/
func (k *KeySets) fetch(ctx context.Context, address, algorithm string) (json.RawMessage, error) {
	if err := checkJWKSURL(address); err != nil {
		return nil, err
	}
	res, err := k.HTTP.New().Timeout(jwksTimeout).MaxBody(jwksMaxBytes).Get(ctx, address)
	if err != nil || res.Status < 200 || res.Status > 299 {
		return nil, ErrJWKSUnreachable
	}
	keys, err := jwt.ParseJWKS(res.Body)
	if err != nil {
		return nil, ErrJWKSUnreachable
	}
	for _, key := range keys {
		if string(key.Algorithm) == algorithm {
			return json.RawMessage(res.Body), nil
		}
	}
	return nil, ErrJWKSInvalid
}

/*
Refresh fetches the secret's key set again and caches it. A failure is
noted on the secret, which keeps its last copy.
*/
func (k *KeySets) Refresh(ctx context.Context, s models.IdentitySecret) (models.IdentitySecret, error) {
	body, fetchErr := k.fetch(ctx, deref(s.JwksUrl), s.Algorithm)
	now := k.Clock.Now()
	var out models.IdentitySecret
	scoped := tenancy.With(ctx, s.WorkspaceID.String())
	err := k.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if fetchErr != nil {
			return q.NoteIdentityJWKSError(ctx, models.NoteIdentityJWKSErrorParams{
				JwksError: new(reason(fetchErr)), Now: now, WorkspaceID: s.WorkspaceID, ID: s.ID,
			})
		}
		var err error
		out, err = q.StoreIdentityJWKS(ctx, models.StoreIdentityJWKSParams{
			JwksCache: body, Now: &now, WorkspaceID: s.WorkspaceID, ID: s.ID,
		})
		return err
	})
	if err != nil {
		return s, fmt.Errorf("store key set: %w", err)
	}
	if fetchErr != nil {
		return s, fetchErr
	}
	return out, nil
}

/*
RefreshDue refreshes up to max key sets of every workspace whose copy is
older than RefreshEvery, returning how many it tried. Each failure is
noted on its secret and does not stop the round.
*/
func (k *KeySets) RefreshDue(ctx context.Context, maxRows int32) (int, error) {
	due, err := models.New(k.DB).ListIdentityJWKSDue(ctx, models.ListIdentityJWKSDueParams{
		StaleBefore: k.Clock.Now().Add(-RefreshEvery), MaxRows: maxRows,
	})
	if err != nil {
		return 0, fmt.Errorf("list key sets to refresh: %w", err)
	}
	for _, row := range due {
		var s models.IdentitySecret
		scoped := tenancy.With(ctx, row.WorkspaceID.String())
		err := k.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
			found, err := models.New(tx).FindIdentitySecret(ctx, models.FindIdentitySecretParams(row))
			s = found
			return err
		})
		if err != nil {
			return 0, fmt.Errorf("load key set secret: %w", err)
		}
		_, _ = k.Refresh(ctx, s)
	}
	return len(due), nil
}

/*
providerKeys returns the keys of the secret's cached set, refreshing it
when the token's kid is missing and the copy is old enough to ask again,
or when the copy is past MaxStale. A copy past MaxStale that cannot be
refreshed verifies nothing.
*/
func (k *KeySets) providerKeys(ctx context.Context, s models.IdentitySecret, kid string,
) (models.IdentitySecret, []jwt.Key, error) {
	now := k.Clock.Now()
	age := MaxStale + time.Second
	if s.JwksFetchedAt != nil {
		age = now.Sub(*s.JwksFetchedAt)
	}
	keys, _ := jwt.ParseJWKS(s.JwksCache)
	missing := !hasKey(keys, kid)
	if age <= MaxStale && (!missing || age < RetryUnknown) {
		return s, keys, nil
	}
	if fresh, ok := k.tryRefresh(ctx, s); ok {
		keys, _ = jwt.ParseJWKS(fresh.JwksCache)
		return fresh, keys, nil
	}
	if age > MaxStale {
		return s, nil, ErrJWKSUnavailable
	}
	return s, keys, nil
}

/*
tryRefresh refreshes the key set, reporting whether a fresh copy came;
why it did not is noted on the secret.
*/
func (k *KeySets) tryRefresh(ctx context.Context, s models.IdentitySecret) (models.IdentitySecret, bool) {
	fresh, err := k.Refresh(ctx, s)
	return fresh, err == nil
}

/*
reason is what the owner reads about a failed fetch.
*/
func reason(err error) string {
	if f, ok := errors.AsType[*fault.Error](err); ok {
		return f.Message()
	}
	return err.Error()
}

func hasKey(keys []jwt.Key, kid string) bool {
	for _, key := range keys {
		if key.ID == kid {
			return true
		}
	}
	return false
}

/*
checkJWKSURL accepts only an https address with a host.
*/
func checkJWKSURL(address string) error {
	u, err := url.Parse(strings.TrimSpace(address))
	if err != nil || u.Scheme != "https" || u.Host == "" || len(address) > 2048 {
		return ErrJWKSURLInvalid
	}
	return nil
}

/*
expectedIssuer is the issuer a provider's tokens must carry: the one the
owner typed, or the key set's address without its well-known path,
which is what Clerk and Supabase use.
*/
func expectedIssuer(s models.IdentitySecret) string {
	if s.JwksIssuer != nil && *s.JwksIssuer != "" {
		return *s.JwksIssuer
	}
	address := deref(s.JwksUrl)
	if i := strings.Index(address, "/.well-known/"); i > 0 {
		return address[:i]
	}
	return ""
}

/*
sameIssuer compares issuers, forgiving a trailing slash, which Auth0
adds and owners often leave off.
*/
func sameIssuer(got, want string) bool {
	return want == "" || strings.TrimSuffix(got, "/") == strings.TrimSuffix(want, "/")
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
