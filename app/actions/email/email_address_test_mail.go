/*
|--------------------------------------------------------------------------
| Forwarding Test
|--------------------------------------------------------------------------
|
| Setting up forwarding happens in another mail system, so the only way
| to know it works is to try it: a test email goes to the public
| address, and when the forward is in place it arrives back at the
| forwarding address carrying the test's own token. That arrival marks
| the address verified and never becomes a conversation. The same email
| proves replies can leave from the address too. Each test is stored and
| times out after ten minutes; tests are spaced out per address and
| counted per workspace, and an install that cannot send still stores
| one so the person can send it from their own mail.
|
*/

package email

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/ratelimit"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	inbound "github.com/dimmihq/dimmi/app/integrations/email"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
	appconfig "github.com/dimmihq/dimmi/config"
)

/*
Limits of forwarding tests: one per address every thirty seconds, and
twenty per workspace a day.
*/
const (
	testSpacing       = 30 * time.Second
	testsPerDay       = 20
	testDay           = 24 * time.Hour
	testSubjectPrefix = "dimmi forwarding test "
)

/*
logTransport is the transport of an install whose customer mail only
goes to its log, which can send no test.
*/
const logTransport = "log"

/*
AddressTest is a forwarding test with where its mail went and the
Message-ID that mail carries, empty before it left.
*/
type AddressTest struct {
	Test      models.EmailAddressTest
	SentTo    string
	MessageID string
}

/*
SendEmailAddressTest stores a forwarding test and mails it to an
address's public address.

Rules:
  - The caller needs email:manage.
  - One test per address every thirty seconds, twenty per workspace a
    day.
  - The test mail carries the test's token in a header and on the last
    line of its body, so its arrival settles this test and no other.
  - Without a mailer, or with one that only writes to the log, the test
    is still stored and waits for the person to send the mail
    themselves.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrEmailTestThrottled] with the wait,
[exceptions.ErrNoSenderAvailable] with the test, the address and the
subject to send by hand, [exceptions.ErrMailUnavailable] when the mailer
refuses it, or a wrapped database error.
*/
type SendEmailAddressTest struct {
	DB        *database.DB
	Clock     clock.Clock
	Mail      *mail.Manager
	Queue     *queue.Queue
	Events    *events.Dispatcher
	Limiter   *ratelimit.Limiter
	Config    appconfig.EmailConfig
	Stamps    inbound.Stamps
	Transport string
}

/*
testStart is a stored test with the mail it sends and the sent email's
row, empty when there is no mailer.
*/
type testStart struct {
	test     models.EmailAddressTest
	message  mail.Message
	outbound models.OutboundEmail
}

/*
Handle stores and sends the test.
*/
func (a *SendEmailAddressTest) Handle(ctx context.Context, m authz.Membership, id ids.UUID) (AddressTest, error) {
	if !m.Can("email:manage") {
		return AddressTest{}, exceptions.ErrForbidden
	}

	ctx = inWorkspace(ctx, m.Workspace.ID)

	var address models.EmailAddress

	err := within(ctx, a.DB, func(ctx context.Context, q *models.Queries) error {
		found, err := findAddress(ctx, q, m.Workspace.ID, id)
		address = found.EmailAddress

		return err
	})
	if err != nil {
		return AddressTest{}, err
	}

	if err := a.throttle(ctx, address); err != nil {
		return AddressTest{}, err
	}

	mailer, mailerErr := a.Mail.Mailer(a.Config.Mailer)
	if mailerErr != nil || a.Transport == logTransport {
		mailer = nil
	}

	start, err := a.start(ctx, m, address, mailer)
	if err != nil {
		return AddressTest{}, err
	}

	if mailer == nil {
		return AddressTest{}, exceptions.ErrNoSenderAvailable.
			With("test", models.PublicID(models.PrefixEmailAddressTest, start.test.ID)).
			With("send_to", address.Address).With("subject", manualTestSubject(start.test.Token))
	}

	test, err := a.send(ctx, mailer, start)
	if err != nil {
		return AddressTest{}, err
	}

	return AddressTest{Test: test, SentTo: address.Address, MessageID: start.outbound.MessageIDHeader}, nil
}

/*
throttle spaces tests of one address and counts the workspace's tests
of the day.
*/
func (a *SendEmailAddressTest) throttle(ctx context.Context, address models.EmailAddress) error {
	limits := []struct {
		key   string
		max   int
		decay time.Duration
	}{
		{"email_test:" + address.ID.String(), 1, testSpacing},
		{"email_test_workspace:" + address.WorkspaceID.String(), testsPerDay, testDay},
	}

	for _, limit := range limits {
		result, err := a.Limiter.Attempt(ctx, limit.key, limit.max, limit.decay)
		if err != nil {
			return fmt.Errorf("limit email address test: %w", err)
		}

		if !result.Allowed {
			wait := max(1, int((result.RetryAfter+time.Second-1)/time.Second))

			return exceptions.ErrEmailTestThrottled.With("retry_after", wait)
		}
	}

	return nil
}

/*
start stores the test, the email it sends when there is a mailer to
send it, and the job that times it out.
*/
func (a *SendEmailAddressTest) start(
	ctx context.Context, m authz.Membership, address models.EmailAddress, mailer mail.Mailer,
) (testStart, error) {
	var start testStart

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		now := a.Clock.Now()
		token := a.Stamps.TestToken(address.ID.String())

		test, err := q.CreateEmailAddressTest(ctx, models.CreateEmailAddressTestParams{
			WorkspaceID: address.WorkspaceID, EmailAddressID: address.ID, Token: token,
			ExpiresAt: now.Add(testWindow), CreatedByMemberID: &m.Member.ID, Now: now,
		})
		if err != nil {
			return fmt.Errorf("record email address test: %w", err)
		}

		start = testStart{test: test, message: a.compose(address, token)}

		if mailer != nil {
			if start.outbound, err = a.save(ctx, q, address, start.message); err != nil {
				return err
			}
		}

		return a.Queue.PushTx(ctx, tx, jobs.CheckEmailAddressTestArgs{
			WorkspaceID: address.WorkspaceID, TestID: test.ID,
		}, queue.At(test.ExpiresAt))
	})

	return start, err
}

/*
compose writes the test email. Its subject names the test, and its last
line repeats the token for forwards that drop extra headers.
*/
func (a *SendEmailAddressTest) compose(address models.EmailAddress, token string) mail.Message {
	domain := domainOf(address.ForwardingAddress)
	explain := "When forwarding from " + address.Address + " is set up, this email arrives back at " +
		address.ForwardingAddress + " and the address shows as receiving."

	return inbound.Compose(inbound.Draft{
		From:      mail.Address{Name: inbound.DisplayName(address.DisplayName), Email: address.ForwardingAddress},
		To:        []mail.Address{{Email: address.Address}},
		Subject:   testSubject(token),
		MessageID: "test." + inbound.NewReplyToken() + "@" + domain,
		Text:      "This is a test from dimmi.\n\n" + explain + "\n\n" + inbound.TestLine(token),
		HTML: "<p>This is a test from dimmi.</p><p>" + explain + "</p><p>" + inbound.TestLine(token) +
			"</p>",
		TestStamp: token,
	})
}

/*
testSubject returns the subject of the test with the token, named by a
short part of the token.
*/
func testSubject(token string) string {
	return testSubjectPrefix + strings.ToUpper(token[3:9])
}

/*
manualTestSubject returns the subject of a test the person sends from
their own mail, which carries the whole token since nothing else of
theirs can.
*/
func manualTestSubject(token string) string {
	return testSubjectPrefix + token
}

/*
sentSubject returns the subject the test's mail left with.
*/
func sentSubject(test models.EmailAddressTest) string {
	if test.OutboundEmailID == nil {
		return manualTestSubject(test.Token)
	}

	return testSubject(test.Token)
}

/*
save records the test email before it is sent.
*/
func (a *SendEmailAddressTest) save(
	ctx context.Context, q *models.Queries, address models.EmailAddress, message mail.Message,
) (models.OutboundEmail, error) {
	row, err := q.CreateOutboundEmail(ctx, models.CreateOutboundEmailParams{
		WorkspaceID: address.WorkspaceID, EmailAddressID: &address.ID, Kind: string(enums.OutboundEmailKindTest),
		FromAddress: message.From.Email, ToAddresses: emailsOf(message.To), CcAddresses: []string{},
		BccAddresses: []string{}, Subject: message.Subject,
		MessageIDHeader: strings.Trim(message.Headers["Message-ID"], "<>"),
		Transport:       a.Transport, Metadata: []byte(`{"stream":"customer"}`), Now: a.Clock.Now(),
	})
	if err != nil {
		return row, fmt.Errorf("record test email: %w", err)
	}

	return row, nil
}

/*
send hands the test to the customer-stream mailer and records the
outcome: the test waits for its mail, or fails when the mailer refused.
*/
func (a *SendEmailAddressTest) send(
	ctx context.Context, mailer mail.Mailer, start testStart,
) (models.EmailAddressTest, error) {
	message := start.message
	message.Tags = map[string]string{"stream": "customer", "kind": string(enums.OutboundEmailKindTest)}

	res, sendErr := mailer.Send(ctx, message)
	if sendErr != nil {
		ctx = context.WithoutCancel(ctx)
	}

	test := start.test
	row := start.outbound
	steps := testSteps{Clock: a.Clock, Events: a.Events}

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		if sendErr != nil {
			if err := q.MarkOutboundEmailFailed(ctx, models.MarkOutboundEmailFailedParams{
				Status: string(enums.OutboundEmailStatusFailed), LastError: new(sendError(sendErr).Error()),
				Now: a.Clock.Now(), WorkspaceID: row.WorkspaceID, ID: row.ID,
			}); err != nil {
				return err
			}

			_, err := steps.settle(ctx, tx, test, testEnd{status: TestFailed})

			return err
		}

		var err error
		test, err = a.waiting(ctx, tx, start, res)

		return err
	})
	if err != nil {
		return test, fmt.Errorf("record email address test outcome: %w", err)
	}

	if sendErr != nil {
		return test, exceptions.ErrMailUnavailable
	}

	return test, nil
}

/*
waiting records the sent email and moves the test to waiting for its
mail, unless the mail came back already.
*/
func (a *SendEmailAddressTest) waiting(
	ctx context.Context, tx *database.Tx, start testStart, res mail.SendResult,
) (models.EmailAddressTest, error) {
	q := models.New(tx)
	row := start.outbound

	if _, err := q.MarkOutboundEmailSent(ctx, models.MarkOutboundEmailSentParams{
		ProviderMessageID: optional(res.ProviderMessageID), Now: a.Clock.Now(),
		WorkspaceID: row.WorkspaceID, ID: row.ID,
	}); err != nil {
		return start.test, err
	}

	moved, err := q.MarkEmailAddressTestWaiting(ctx, models.MarkEmailAddressTestWaitingParams{
		OutboundEmailID: &row.ID, Now: a.Clock.Now(), WorkspaceID: row.WorkspaceID, ID: start.test.ID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return start.test, nil
	}

	if err != nil {
		return start.test, err
	}

	return moved, testSteps{Clock: a.Clock, Events: a.Events}.announce(ctx, tx, moved)
}
