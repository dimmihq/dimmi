package auth_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/testkit"

	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
linkPattern finds the web addresses in a mail body.
*/
var linkPattern = regexp.MustCompile(`https?://[^\s<>"]+`)

/*
everyAccountMail runs every flow that mails a user about their account
and returns the messages the fake recorded.
*/
func everyAccountMail(t *testing.T, tc *tests.TestCase) []mail.SentMessage {
	t.Helper()

	fake := tc.Mail()
	tc.Client().PostJSON("/api/v1/users", registration("grace@example.com")).AssertCreated()
	tc.Client().PostJSON("/api/v1/magic-links", testkit.JSON{"email": tc.Owner.Email}).
		AssertStatus(fhttp.StatusAccepted)
	id, token := resetLink(t, tc, tc.Owner.Email)
	redeem(tc.Client(), id, token).AssertOK()
	tc.Client().PostJSON("/api/v1/sessions", login(tc.Owner.Email, newPassword)).AssertCreated()
	signInByCode(t, tc, tc.CreateUser(factories.WithoutPassword)).
		PostJSON("/api/v1/user/confirmation-codes", nil).AssertStatus(fhttp.StatusAccepted)
	tc.Drain()

	return fake.Sent()
}

func TestAccountMail(t *testing.T) {
	t.Parallel()

	t.Run("sends every account mail with a text and an HTML body, its secrets after the # only", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		sent := everyAccountMail(t, tc)

		kinds := map[string]bool{}
		for _, s := range sent {
			kinds[strings.TrimPrefix(fmt.Sprintf("%T", s.Mailable), "mail.")] = true
			assert.NotEmpty(t, s.Message.Subject)
			assert.NotEmpty(t, s.Message.Text)
			assert.NotEmpty(t, s.Message.HTML)
			assert.Equal(t, "no-reply@localhost", s.Message.From.Email)
			assert.NotRegexp(t, `\d{3}-\d{3}`, s.Message.Subject)

			for _, link := range linkPattern.FindAllString(s.Message.Text, -1) {
				address, _, _ := strings.Cut(link, "#")
				assert.NotContains(t, address, "token", link)

				if secret := tc.Fragment(link).Get("token"); secret != "" {
					assert.NotContains(t, s.Message.Subject, secret)
				}
			}
		}

		for _, kind := range []string{
			"VerifyEmailMail", "MagicLinkMail", "ResetPasswordMail", "PasswordChangedMail", "NewSignInMail",
			"ConfirmCodeMail",
		} {
			assert.True(t, kinds[kind], "no %s among %v", kind, kinds)
		}
	})

	t.Run("sends from the address the operator chose", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_MAIL__FROM__EMAIL=support@acme.test", "DIMMI_MAIL__FROM__NAME=Acme"))
		fake := tc.Mail()

		tc.Client().PostJSON("/api/v1/password-resets", testkit.JSON{"email": tc.Owner.Email}).
			AssertStatus(fhttp.StatusAccepted)

		_, message := tests.LastSent[appmail.ResetPasswordMail](tc, fake)
		assert.Equal(t, "support@acme.test", message.Message.From.Email)
		assert.Equal(t, "Acme", message.Message.From.Name)
	})

	t.Run("writes a user's mail in the language they chose", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, err := tc.Operator().Exec(t.Context(), "UPDATE users SET locale = 'fr' WHERE id = $1", tc.Owner.ID)
		require.NoError(t, err)
		fake := tc.Mail()

		tc.Client().PostJSON("/api/v1/password-resets", testkit.JSON{"email": tc.Owner.Email}).
			AssertStatus(fhttp.StatusAccepted)

		_, message := tests.LastSent[appmail.ResetPasswordMail](tc, fake)
		assert.Equal(t, "fr", message.Locale)
		assert.Equal(t, []string{tc.Owner.Email}, tests.SentTo(message))
	})

	t.Run("mails a sign-in from a new device, never the first one or a fresh one", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()

		first := tc.ActingAsOwner()
		tc.Drain()
		assert.Empty(t, newSignIns(tc, fake), "the first sign-in")

		tc.ActingAsOwner()
		assert.Len(t, newSignIns(tc, fake), 1, "a second browser")

		first.PostJSON("/api/v1/sessions", testkit.JSON{
			"grant": "password", "email": tc.Owner.Email, "password": factories.Password, "reauth": true,
		}).AssertCreated()
		assert.Len(t, newSignIns(tc, fake), 1, "a fresh sign-in on the same session")

		mail, message := tests.LastSent[appmail.NewSignInMail](tc, fake)
		assert.Equal(t, "192.0.2.1", mail.IP)
		assert.Contains(t, mail.SecurityURL, "/d/settings/security")
		assert.Contains(t, message.Message.Text, "signed in from an unknown device at 192.0.2.1")
	})

	t.Run("names the browser a new sign-in came from", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		tc.ActingAsOwner()
		chrome := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) " +
			"Chrome/120.0.0.0 Safari/537.36"

		tc.Client().WithHeader("User-Agent", chrome).
			PostJSON("/api/v1/sessions", login(tc.Owner.Email, factories.Password)).AssertCreated()

		_, message := tests.LastSent[appmail.NewSignInMail](tc, fake)
		assert.Contains(t, message.Message.Text, "signed in from Chrome on MacOS at 192.0.2.1")
	})

	t.Run("sends no new-device mail for the sign-in that comes with signing up", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()

		tc.Client().PostJSON("/api/v1/users", registration("grace@example.com")).AssertCreated()

		assert.Empty(t, newSignIns(tc, fake))
	})

	t.Run("warns the owner once a day when strangers lock their account", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv(behindProxy, "DIMMI_AUTH__LOCKOUT__PER_EMAIL_SOFT_LOCK=3"))
		fake := tc.Mail()

		for _, ip := range []string{"198.51.100.1", "198.51.100.2", "198.51.100.3"} {
			fail(t, from(tc, ip), tc.Owner.Email, 1)
		}
		tc.Clock.Advance(16 * time.Minute)
		fail(t, from(tc, "198.51.100.4"), tc.Owner.Email, 1)
		fail(t, tc.Client(), "nobody@example.com", 3)

		warnings, messages := tests.Sent[appmail.SuspiciousSignInAttemptsMail](tc, fake)
		require.Len(t, warnings, 1)
		assert.Equal(t, []string{tc.Owner.Email}, tests.SentTo(messages[0]))
	})
}

/*
newSignIns drains the queue and returns the new-device mails sent.
*/
func newSignIns(tc *tests.TestCase, fake *mail.Fake) []appmail.NewSignInMail {
	sent, _ := tests.Sent[appmail.NewSignInMail](tc, fake)

	return sent
}

/*
signInByCode signs a user in with a mailed code and returns the
browser.
*/
func signInByCode(t *testing.T, tc *tests.TestCase, user models.User) *testkit.Client {
	t.Helper()

	sent := magicLink(t, tc, user.Email, nil)

	return tc.Browser(withCode(tc.Client(), user.Email, sent.Code).AssertCreated())
}

/*
fallbackLine returns the log line of the one mail written to the log.
*/
func fallbackLine(t *testing.T, logs *syncBuffer) map[string]any {
	t.Helper()

	scanner := bufio.NewScanner(strings.NewReader(logs.String()))
	scanner.Buffer(nil, 1<<20)

	for scanner.Scan() {
		var line map[string]any
		if json.Unmarshal(scanner.Bytes(), &line) == nil && line["mail_fallback"] == true {
			return line
		}
	}

	t.Fatalf("no mail line in the log:\n%s", logs.String())

	return nil
}

/*
syncBuffer is a log writer the application's goroutines may write to
while the test reads it.
*/
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}
