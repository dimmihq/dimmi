/*
|--------------------------------------------------------------------------
| Reader Email Codes
|--------------------------------------------------------------------------
|
| A reader can sign in to a help center with nothing but their email: a
| six-digit code goes to the address and they type it back. The code
| lives ten minutes, kept only as a hash bound to the help center and the
| address, and five wrong tries burn it. Sending is throttled per address
| and per network. A good code proves the address and nothing more: the
| reader is the customer the team's product named with it, else a lead
| with that address, now verified. Nobody is merged.
|
*/

package helpcenters

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/support/str"

	"github.com/dimmihq/dimmi/app/actions/widget"
	appauth "github.com/dimmihq/dimmi/app/auth"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
)

/*
How long a reader's code works, how many wrong tries it takes and how
soon another may be sent.
*/
const (
	ReaderCodeTTL     = 10 * time.Minute
	ReaderCodeTries   = 5
	ReaderResendAfter = 30 * time.Second
)

/*
Failures of signing in with a code, shown to the reader.
*/
var (
	ErrReaderEmailInvalid = fault.New(fault.Invalid, "email_invalid",
		"Enter an email address such as you@example.com.").OnField("email")
	ErrReaderCodeExpired = fault.New(fault.Gone, "reader_code_expired",
		"That code expired. Send a new one.")
	ErrReaderCodeInvalid = fault.New(fault.Invalid, "reader_code_invalid",
		"That code didn't work. Check it and try again.").OnField("code")
	ErrReaderCodeLocked = fault.New(fault.RateLimited, "reader_code_locked",
		"Too many wrong codes. Send a new one.")
	ErrEmailSignInOff = fault.New(fault.NotFound, "email_sign_in_off",
		"This help center does not take email sign-in.")
)

/*
CodeRequest is a reader asking for a code: the help center, the address
they typed and the network they asked from, empty when unknown.
*/
type CodeRequest struct {
	Center  models.HelpCenter
	Email   string
	Network string
}

/*
SendCode mails a fresh code to the address, replacing any earlier one,
and returns the address as the code page repeats it, masked.
*/
func (r *Readers) SendCode(ctx context.Context, in CodeRequest) (string, error) {
	if !SettingsOf(in.Center).EmailSignInOn() {
		return "", ErrEmailSignInOff
	}

	email, ok := widget.NormalizeTypedEmail(in.Email)
	if !ok {
		return "", ErrReaderEmailInvalid
	}

	if err := r.throttle(ctx, in, email); err != nil {
		return "", err
	}

	code := str.RandomDigits(6)
	key := codeKey(in.Center, email)
	if err := r.Cache.Set(ctx, key, []byte(codeHash(key, code)), ReaderCodeTTL); err != nil {
		return "", fmt.Errorf("keep reader code: %w", err)
	}

	if err := r.Cache.Delete(ctx, "tries:"+key); err != nil {
		return "", fmt.Errorf("reset reader code tries: %w", err)
	}

	err := r.Mail.To(mail.Address{Email: email}).
		Queue(ctx, appmail.ReaderCodeMail{HelpCenter: in.Center.Name, Code: code})
	if err != nil {
		return "", fmt.Errorf("mail reader code: %w", err)
	}

	return widget.MaskEmail(email), nil
}

/*
throttle limits codes: one per 30 seconds and five an hour per address
of the help center, twenty an hour per network.
*/
func (r *Readers) throttle(ctx context.Context, in CodeRequest, email string) error {
	address := in.Center.ID.String() + ":" + hashing.SHA256Hex([]byte(email))
	rates := []appauth.Rate{
		{Key: "help_reader_code:burst:" + address, Limit: 1, Window: ReaderResendAfter},
		{Key: "help_reader_code:address:" + address, Limit: 5, Window: time.Hour},
	}

	if in.Network != "" {
		rates = append(rates, appauth.Rate{Key: "help_reader_code:network:" + in.Network, Limit: 20, Window: time.Hour})
	}

	return appauth.Throttle(ctx, r.Limiter, rates...)
}

/*
CheckCode takes the code a reader typed for the address. A good one
signs them in: it returns the session that remembers them.
*/
func (r *Readers) CheckCode(ctx context.Context, h models.HelpCenter, email, code string) (ReaderSession, error) {
	email, ok := widget.NormalizeTypedEmail(email)
	if !ok {
		return ReaderSession{}, ErrReaderEmailInvalid
	}

	if err := r.spendCode(ctx, codeKey(h, email), strings.TrimSpace(code)); err != nil {
		return ReaderSession{}, err
	}

	contact, err := r.byEmail(ctx, h, email)
	if err != nil {
		return ReaderSession{}, err
	}

	return r.session(h, contact), nil
}

/*
spendCode checks a code against the one kept under the key, counting
the try, and removes it once it matched.
*/
func (r *Readers) spendCode(ctx context.Context, key, code string) error {
	kept, err := r.Cache.Get(ctx, key)
	if errors.Is(err, cache.ErrMiss) {
		return ErrReaderCodeExpired
	}

	if err != nil {
		return fmt.Errorf("load reader code: %w", err)
	}

	tries, err := r.Cache.Increment(ctx, "tries:"+key, 1, ReaderCodeTTL)
	if err != nil {
		return fmt.Errorf("count reader code tries: %w", err)
	}

	if tries > ReaderCodeTries {
		_ = r.Cache.Delete(ctx, key)
		return ErrReaderCodeLocked
	}

	if !hashing.Equal([]byte(codeHash(key, code)), kept) {
		return ErrReaderCodeInvalid.With("attempts_left", ReaderCodeTries-tries)
	}

	return wrapErr("spend reader code", r.Cache.Delete(ctx, key))
}

func codeKey(h models.HelpCenter, email string) string {
	return "help:reader_code:" + h.ID.String() + ":" + hashing.SHA256Hex([]byte(email))
}

func codeHash(key, code string) string {
	return hashing.SHA256Hex([]byte(key + ":" + code))
}

func wrapErr(doing string, err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("%s: %w", doing, err)
}
