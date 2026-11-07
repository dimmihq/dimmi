/*
|--------------------------------------------------------------------------
| Email Code Proof
|--------------------------------------------------------------------------
|
| A visitor who never signed in can still prove they own an email
| address: the messenger mails a six-digit code and the visitor types it
| into a card. The code never travels as a chat message, so it never
| reaches the transcript or the agent. It lives ten minutes, stored only
| as a hash bound to the session, and five wrong tries burn it. Sending
| is throttled per session, per address and per network. A good code
| marks the session's email proven for a day and the lead's address
| verified; the actions waiting for that proof move on.
|
*/

package widget

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/ratelimit"
	"github.com/dimmihq/dimmi/framework/support/str"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
)

/*
How long a code works and how many wrong tries it takes, how long a
proof lasts, and how soon another code may be sent.
*/
const (
	EmailCodeTTL       = 10 * time.Minute
	EmailCodeTries     = 5
	EmailProofLifetime = 24 * time.Hour
	EmailResendAfter   = 30 * time.Second
)

/*
Failures of proving an email, shown to the visitor.
*/
var (
	ErrEmailCodeExpired = fault.New(fault.Gone, "email_code_expired",
		"That code expired. Send a new one.")
	ErrEmailCodeInvalid = fault.New(fault.Invalid, "email_code_invalid",
		"That code didn't work. Check it and try again.")
	ErrEmailCodeLocked = fault.New(fault.RateLimited, "email_code_locked",
		"Too many wrong codes. Send a new one.")
	ErrAlreadySignedIn = fault.New(fault.Conflict, "identity_already_verified",
		"You are signed in already; there is nothing to confirm.")
)

/*
EmailProofs sends and checks email codes.
*/
type EmailProofs struct {
	DB      *database.DB
	Events  *events.Dispatcher
	Clock   clock.Clock
	Cache   *cache.Store
	Limiter *ratelimit.Limiter
	Mail    *mail.Manager
}

/*
SentCode is a code on its way: the address masked as the card shows
it, when the code stops working and when another may be sent.
*/
type SentCode struct {
	Email       string
	ExpiresAt   time.Time
	ResendAfter time.Time
}

/*
pendingCode is what is kept of a sent code: the address and the code's
hash.
*/
type pendingCode struct {
	Email string `json:"email"`
	Hash  string `json:"hash"`
}

/*
Send mails a fresh code to the address, replacing any earlier one.
*/
func (a *EmailProofs) Send(ctx context.Context, auth Authenticated, raw string) (SentCode, error) {
	s := auth.Session
	if s.Kind == string(enums.WidgetSessionIdentified) {
		return SentCode{}, ErrAlreadySignedIn
	}
	email, ok := normalizeTypedEmail(raw)
	if !ok {
		return SentCode{}, contactField("email", "Enter an email address such as you@example.com.")
	}
	if err := a.throttle(ctx, s, email); err != nil {
		return SentCode{}, err
	}
	code, now := str.RandomDigits(6), a.Clock.Now()
	stored, err := json.Marshal(pendingCode{Email: email, Hash: codeHash(s.ID, code)})
	if err != nil {
		return SentCode{}, fmt.Errorf("encode email code: %w", err)
	}
	if err := a.Cache.Set(ctx, codeKey(s.ID), stored, EmailCodeTTL); err != nil {
		return SentCode{}, fmt.Errorf("keep email code: %w", err)
	}
	if err := a.Cache.Delete(ctx, triesKey(s.ID)); err != nil {
		return SentCode{}, fmt.Errorf("reset email code tries: %w", err)
	}
	pending := a.Mail.To(mail.Address{Email: email})
	if s.Locale != nil {
		pending.Locale(*s.Locale)
	}
	err = pending.Queue(ctx, appmail.VisitorCodeMail{Workspace: auth.Resolved.Workspace.Name, Code: code})
	if err != nil {
		return SentCode{}, fmt.Errorf("mail email code: %w", err)
	}
	return SentCode{Email: MaskEmail(email), ExpiresAt: now.Add(EmailCodeTTL), ResendAfter: now.Add(EmailResendAfter)},
		nil
}

/*
throttle limits codes: one per 30 seconds and five an hour per session,
five an hour per address in the workspace, twenty an hour per network.
*/
func (a *EmailProofs) throttle(ctx context.Context, s models.WidgetSession, email string) error {
	rates := []appauth.Rate{
		{Key: "widget_email_code:burst:" + s.ID.String(), Limit: 1, Window: EmailResendAfter},
		{Key: "widget_email_code:session:" + s.ID.String(), Limit: 5, Window: time.Hour},
		{
			Key:   "widget_email_code:address:" + s.WorkspaceID.String() + ":" + hashing.SHA256Hex([]byte(email)),
			Limit: 5, Window: time.Hour,
		},
	}
	if s.IpPrefix != nil {
		rates = append(rates, appauth.Rate{
			Key: "widget_email_code:network:" + s.IpPrefix.String(), Limit: 20,
			Window: time.Hour,
		})
	}
	return appauth.Throttle(ctx, a.Limiter, rates...)
}

/*
Check takes the code the visitor typed. A good one proves the session's
email and verifies the lead's address.
*/
func (a *EmailProofs) Check(ctx context.Context, auth Authenticated, code string) (models.WidgetSession, error) {
	s := auth.Session
	pending, err := a.pending(ctx, s.ID)
	if err != nil {
		return s, err
	}
	tries, err := a.Cache.Increment(ctx, triesKey(s.ID), 1, EmailCodeTTL)
	if err != nil {
		return s, fmt.Errorf("count email code tries: %w", err)
	}
	if tries > EmailCodeTries {
		_ = a.Cache.Delete(ctx, codeKey(s.ID))
		return s, ErrEmailCodeLocked
	}
	if !hashing.Equal([]byte(codeHash(s.ID, strings.TrimSpace(code))), []byte(pending.Hash)) {
		return s, ErrEmailCodeInvalid.With("attempts_left", EmailCodeTries-tries)
	}
	if err := a.Cache.Delete(ctx, codeKey(s.ID)); err != nil {
		return s, fmt.Errorf("spend email code: %w", err)
	}
	return a.prove(ctx, s, pending.Email)
}

/*
pending loads the session's code, or says it expired.
*/
func (a *EmailProofs) pending(ctx context.Context, session ids.UUID) (pendingCode, error) {
	raw, err := a.Cache.Get(ctx, codeKey(session))
	if errors.Is(err, cache.ErrMiss) {
		return pendingCode{}, ErrEmailCodeExpired
	}
	if err != nil {
		return pendingCode{}, fmt.Errorf("load email code: %w", err)
	}
	var out pendingCode
	if err := json.Unmarshal(raw, &out); err != nil {
		return pendingCode{}, ErrEmailCodeExpired
	}
	return out, nil
}

/*
prove records the proof on the session and the lead in one transaction
and tells who listens.
*/
func (a *EmailProofs) prove(ctx context.Context, s models.WidgetSession, email string) (models.WidgetSession, error) {
	now := a.Clock.Now()
	var out models.WidgetSession
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		var err error
		out, err = q.ProveWidgetSessionEmail(ctx, models.ProveWidgetSessionEmailParams{
			Email: &email, Now: now, WorkspaceID: s.WorkspaceID, ID: s.ID,
		})
		if err != nil {
			return fmt.Errorf("record email proof: %w", err)
		}
		if _, err := q.ProveContactEmail(ctx, models.ProveContactEmailParams{
			Email: &email, Now: now, WorkspaceID: s.WorkspaceID, ID: s.ContactID,
		}); err != nil {
			return fmt.Errorf("verify lead email: %w", err)
		}
		return a.Events.Dispatch(ctx, tx, appevents.WidgetEmailProven{
			WorkspaceID: s.WorkspaceID, ContactID: s.ContactID, SessionID: s.ID, Email: email,
		})
	})
	return out, err
}

/*
EmailProven reports whether the session proved an email recently enough
to still count.
*/
func EmailProven(s models.WidgetSession, now time.Time) bool {
	return s.EmailVerifiedAt != nil && now.Sub(*s.EmailVerifiedAt) < EmailProofLifetime
}

func codeHash(session ids.UUID, code string) string {
	return hashing.SHA256Hex([]byte(session.String() + ":" + code))
}

func codeKey(session ids.UUID) string {
	return "widget:email_code:" + session.String()
}

func triesKey(session ids.UUID) string {
	return "widget:email_code_tries:" + session.String()
}

/*
MaskEmail shows an address the way the card repeats it: its first
letter and its domain.
*/
func MaskEmail(email string) string {
	at := strings.LastIndex(email, "@")
	if at < 1 {
		return email
	}
	return email[:1] + "***" + email[at:]
}
