/*
|--------------------------------------------------------------------------
| One-Time Tokens
|--------------------------------------------------------------------------
|
| Short-lived secrets sent by mail: sign-in links and codes, email
| verifications, confirmation codes. Each is spent once, by one UPDATE
| that also checks it is unspent and unexpired, so two requests racing
| for one secret can never both win. Five wrong guesses burn it.
|
| Only a few may be live at once for one address and purpose: asking
| again ends the oldest, so several open tabs keep working while an
| attacker never has more than a handful of codes to guess at.
|
*/

package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
MaxSecretAttempts is how many wrong guesses burn a mailed secret.
*/
const MaxSecretAttempts = 5

/*
OneTimeTokens issues, finds and spends one-time tokens.
*/
type OneTimeTokens struct {
	DB    *database.DB
	Clock clock.Clock
	Codes Codes
}

/*
Issue is a secret to send. UserID is nil when the address has no
account yet. Data is stored with the secret. WithCode adds a six-digit
code beside the link. Live caps the secrets of the purpose that may be
live for the address at once, the oldest ending first.
*/
type Issue struct {
	Purpose  enums.OneTimePurpose
	UserID   *ids.UUID
	Email    string
	Data     map[string]any
	Client   Client
	TTL      time.Duration
	WithCode bool
	Live     int32
}

/*
Issued is a secret just stored, with its plain token and code, which
exist nowhere else and go straight into the mail.
*/
type Issued struct {
	ID        ids.UUID
	Token     string
	Code      string
	ExpiresAt time.Time
}

/*
Issue stores the secret, inside the transaction in the context when
there is one, and ends the oldest ones over the cap. Its data holds only
strings, which always encode, so that error is not checked.
*/
func (t *OneTimeTokens) Issue(ctx context.Context, in Issue) (Issued, error) {
	now := t.Clock.Now()
	plain, digest := NewSecret()
	issued := Issued{ID: ids.NewV7At(now), Token: plain, ExpiresAt: now.Add(in.TTL)}

	var sealed []byte
	if in.WithCode {
		issued.Code = t.Codes.New()
		sealed = t.Codes.Seal(issued.ID, issued.Code)
	}

	data, _ := json.Marshal(in.Data)
	if in.Data == nil {
		data = []byte("{}")
	}

	q := models.New(t.DB)

	_, err := q.CreateOneTimeToken(ctx, models.CreateOneTimeTokenParams{
		ID: issued.ID, Purpose: string(in.Purpose), UserID: in.UserID, Email: in.Email, TokenHash: digest,
		CodeHash: sealed, Data: data, Ip: Address(in.Client.IP), UserAgent: optional(in.Client.TruncatedUserAgent()),
		ExpiresAt: issued.ExpiresAt, Now: now,
	})
	if err != nil {
		return Issued{}, fmt.Errorf("store %s token: %w", in.Purpose, err)
	}

	err = q.ExpireOlderOneTimeTokens(ctx, models.ExpireOlderOneTimeTokensParams{
		Email: in.Email, Purpose: string(in.Purpose), Now: now, Keep: in.Live,
	})
	if err != nil {
		return Issued{}, fmt.Errorf("end older %s tokens: %w", in.Purpose, err)
	}

	return issued, nil
}

/*
Find returns the secret of the purpose with the id, spent or not, and
whether there is one.
*/
func (t *OneTimeTokens) Find(ctx context.Context, purpose enums.OneTimePurpose, id ids.UUID) (
	models.OneTimeToken, bool, error,
) {
	tok, err := models.New(t.DB).FindOneTimeToken(ctx, models.FindOneTimeTokenParams{ID: id, Purpose: string(purpose)})

	return found(tok, err, purpose)
}

/*
FindByToken returns the secret of the purpose whose plain token this
is, spent or not, and whether there is one.
*/
func (t *OneTimeTokens) FindByToken(ctx context.Context, purpose enums.OneTimePurpose, plain string) (
	models.OneTimeToken, bool, error,
) {
	tok, err := models.New(t.DB).FindOneTimeTokenByHash(ctx, models.FindOneTimeTokenByHashParams{
		TokenHash: Digest(plain), Purpose: string(purpose),
	})

	return found(tok, err, purpose)
}

/*
Proves reports, in constant time, whether the plain token is the
secret's own.
*/
func (t *OneTimeTokens) Proves(tok models.OneTimeToken, plain string) bool {
	return hashing.Equal(Digest(plain), tok.TokenHash)
}

/*
Spend uses the secret up and reports whether it was still usable. Of two
requests racing for one secret, only one is told it was.
*/
func (t *OneTimeTokens) Spend(ctx context.Context, tok models.OneTimeToken) (bool, error) {
	_, err := models.New(t.DB).SpendOneTimeToken(ctx, models.SpendOneTimeTokenParams{
		ID: tok.ID, Now: t.Clock.Now(), MaxAttempts: MaxSecretAttempts,
	})
	if errors.Is(err, database.ErrNotFound) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("spend %s token: %w", tok.Purpose, err)
	}

	return true, nil
}

/*
Miss counts a wrong guess at the secret.
*/
func (t *OneTimeTokens) Miss(ctx context.Context, tok models.OneTimeToken) error {
	if _, err := models.New(t.DB).CountOneTimeTokenAttempt(ctx, tok.ID); err != nil {
		return fmt.Errorf("count wrong guess at %s token: %w", tok.Purpose, err)
	}

	return nil
}

/*
MatchCode returns the live secret of the purpose sent to the address
whose code this is, and whether one is. Every live secret is compared,
and a wrong code counts as a wrong guess at each of them, so guessing
against several live secrets burns them all together.
*/
func (t *OneTimeTokens) MatchCode(ctx context.Context, purpose enums.OneTimePurpose, email, code string) (
	models.OneTimeToken, bool, error,
) {
	live, err := models.New(t.DB).ListLiveOneTimeTokens(ctx, models.ListLiveOneTimeTokensParams{
		Email: email, Purpose: string(purpose), Now: t.Clock.Now(), MaxAttempts: MaxSecretAttempts,
	})
	if err != nil {
		return models.OneTimeToken{}, false, fmt.Errorf("list live %s tokens: %w", purpose, err)
	}

	var (
		match models.OneTimeToken
		hit   bool
	)

	for _, tok := range live {
		if t.Codes.Matches(tok.ID, code, tok.CodeHash) {
			match, hit = tok, true
		}
	}

	if hit {
		return match, true, nil
	}

	for _, tok := range live {
		if err := t.Miss(ctx, tok); err != nil {
			return models.OneTimeToken{}, false, err
		}
	}

	return models.OneTimeToken{}, false, nil
}

/*
found turns a lookup into the secret and whether there is one.
*/
func found(tok models.OneTimeToken, err error, purpose enums.OneTimePurpose) (models.OneTimeToken, bool, error) {
	if errors.Is(err, database.ErrNotFound) {
		return models.OneTimeToken{}, false, nil
	}

	if err != nil {
		return models.OneTimeToken{}, false, fmt.Errorf("find %s token: %w", purpose, err)
	}

	return tok, true, nil
}
