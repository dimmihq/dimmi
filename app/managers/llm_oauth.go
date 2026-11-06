/*
|--------------------------------------------------------------------------
| Subscription Token Renewal
|--------------------------------------------------------------------------
|
| A ChatGPT sign-in's refresh token rotates: spending it twice logs the
| member out. So one renewal runs at a time per connection — a lock in
| this process and a row lock across processes — and each renewal first
| re-reads the row, taking a token another worker already renewed
| instead of spending the refresh token again. The renewed credential
| is saved in its own transaction, outside the caller's, so a run that
| later fails can never roll the new refresh token back; only the
| credential column is written, so an edit to the connection's name or
| endpoint made meanwhile is never overwritten.
|
*/

package managers

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/llm/chatgpt"
	"github.com/dimmihq/dimmi/app/models"
)

/*
renewBefore is how long before expiry a bearer is renewed.
*/
const renewBefore = 5 * time.Minute

/*
ChatGPTClient is the subscription's sign-in client, as configured.
*/
func (m *LLMManager) ChatGPTClient() *chatgpt.Client {
	cfg := m.config.ChatGPT
	return &chatgpt.Client{HTTP: m.http, ClientID: cfg.ClientID, Issuer: cfg.Issuer, TokenURL: cfg.TokenURL}
}

/*
oauthSource is the chatgpt TokenSource bound to one key row. Its lock
makes concurrent callers of one driver wait for a single renewal.
*/
type oauthSource struct {
	m     *LLMManager
	key   models.AiProviderKey
	mu    sync.Mutex
	creds Credentials
}

/*
Token returns the bearer, renewing it when near expiry.
*/
func (s *oauthSource) Token(ctx context.Context) (chatgpt.Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.m.fresh(s.creds) {
		return bearer(s.creds), nil
	}
	return s.renewLocked(ctx, "")
}

/*
Renew returns a bearer other than the one the endpoint refused.
*/
func (s *oauthSource) Renew(ctx context.Context) (chatgpt.Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.renewLocked(ctx, s.creds.AccessToken)
}

/*
renewLocked renews the held credential, skipping the refused bearer.
*/
func (s *oauthSource) renewLocked(ctx context.Context, refused string) (chatgpt.Token, error) {
	creds, err := s.m.renew(ctx, s.key, refused)
	if err != nil {
		return chatgpt.Token{}, err
	}
	s.creds = creds
	return bearer(creds), nil
}

/*
bearer is the token the driver sends.
*/
func bearer(creds Credentials) chatgpt.Token {
	return chatgpt.Token{AccessToken: creds.AccessToken, AccountID: creds.AccountID}
}

/*
fresh reports a bearer that stays valid past the renewal margin.
*/
func (m *LLMManager) fresh(creds Credentials) bool {
	return creds.AccessToken != "" && m.clock.Now().Before(creds.ExpiresAt.Add(-renewBefore))
}

/*
renew returns a usable credential for the key: the stored one when it
is fresh and not the bearer the endpoint refused, which covers a
renewal another worker already made, or a freshly refreshed and saved
one.
*/
func (m *LLMManager) renew(ctx context.Context, key models.AiProviderKey, refused string) (Credentials, error) {
	if m.db == nil {
		return Credentials{}, signInError("subscription refresh is unavailable")
	}

	lock := m.renewLock(key.ID)
	lock.Lock()
	defer lock.Unlock()

	var out Credentials
	scoped := tenancy.With(database.Detach(ctx), key.WorkspaceID.String())
	err := m.db.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		row, err := models.New(tx).LockProviderKeyCredentials(ctx, models.LockProviderKeyCredentialsParams{
			WorkspaceID: key.WorkspaceID, ID: key.ID,
		})
		if err != nil {
			return signInError("the subscription connection is gone")
		}
		current, err := OpenCredentials(ctx, m.keys, row)
		if err != nil {
			return err
		}
		if m.fresh(current) && current.AccessToken != refused {
			out = current
			return nil
		}
		out, err = m.refresh(ctx, tx, row, current)
		return err
	})
	return out, err
}

/*
refresh spends the refresh token and saves the renewed credential.
*/
func (m *LLMManager) refresh(ctx context.Context, tx *database.Tx, row models.AiProviderKey,
	current Credentials,
) (Credentials, error) {
	set, err := m.ChatGPTClient().Refresh(ctx, chatgpt.TokenSet{
		AccessToken: current.AccessToken, RefreshToken: current.RefreshToken,
		IDToken: current.IDToken, AccountID: current.AccountID,
	}, m.clock.Now())
	if errors.Is(err, chatgpt.ErrSignInExpired) {
		return Credentials{}, signInError("the ChatGPT sign-in expired; connect it again")
	}
	if err != nil {
		return Credentials{}, &contracts.LLMError{
			Kind: contracts.LLMErrorNetwork, Message: "subscription token refresh failed",
		}
	}

	current.AccessToken, current.RefreshToken = set.AccessToken, set.RefreshToken
	current.IDToken, current.ExpiresAt = set.IDToken, set.ExpiresAt
	if set.AccountID != "" {
		current.AccountID = set.AccountID
	}

	sealed, err := SealCredentials(ctx, m.keys, row, current)
	if err != nil {
		return Credentials{}, err
	}
	if _, err := models.New(tx).UpdateProviderKeyCredentials(ctx, models.UpdateProviderKeyCredentialsParams{
		WorkspaceID: row.WorkspaceID, ID: row.ID, CredentialsEncrypted: sealed, Now: m.clock.Now(),
	}); err != nil {
		return Credentials{}, fmt.Errorf("store renewed credentials: %w", err)
	}
	return current, nil
}

/*
renewLock returns the in-process lock serializing one key's renewals.
*/
func (m *LLMManager) renewLock(keyID ids.UUID) *sync.Mutex {
	m.mu.Lock()
	defer m.mu.Unlock()
	lock, ok := m.renewing[keyID]
	if !ok {
		lock = &sync.Mutex{}
		m.renewing[keyID] = lock
	}
	return lock
}

/*
signInError is a subscription failure the member fixes by connecting
again.
*/
func signInError(message string) error {
	return &contracts.LLMError{Kind: contracts.LLMErrorAuth, ProviderCode: "subscription_sign_in", Message: message}
}
