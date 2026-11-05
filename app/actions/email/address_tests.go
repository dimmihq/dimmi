/*
|--------------------------------------------------------------------------
| Forwarding Test Outcomes
|--------------------------------------------------------------------------
|
| A forwarding test ends exactly once: its mail comes back, bounces, or
| is not back within ten minutes. Whichever happens first settles it and
| the rest find it settled and change nothing. Every step is announced
| so an open settings page follows along. What the test learned on the
| way, such as a slow forward or a mail service refusing to forward out,
| is kept on it as findings in plain codes.
|
*/

package email

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The states of a forwarding test.
*/
const (
	TestSending  = "sending"
	TestWaiting  = "waiting"
	TestReceived = "received"
	TestFailed   = "failed"
	TestTimedOut = "timed_out"
)

/*
Timing of forwarding tests: how long one waits for its mail, after how
long a forward counts as slow, and how long tests are kept.
*/
const (
	testWindow    = 10 * time.Minute
	testSlowAfter = 5 * time.Minute
	testRetention = 30 * 24 * time.Hour
	testEarlyRun  = time.Minute
)

/*
ignoreTestStale is why a test mail is dropped when its test is over.
*/
const ignoreTestStale = "test_stale"

/*
TestFinding is something a forwarding test learned about the forward:
a code with its own plain message, and how much it matters (error, warn
or info).
*/
type TestFinding struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
}

/*
testEnd is how a test ends: its final status, the mail that came back,
what was learned, and how long the mail took.
*/
type testEnd struct {
	status    string
	inboundID *ids.UUID
	findings  []TestFinding
	latency   *int32
}

/*
testSteps settles and announces tests.
*/
type testSteps struct {
	Clock  clock.Clock
	Events *events.Dispatcher
}

/*
settle ends a test that is still open and announces it. A test already
settled stays as it was, and settle reports false.
*/
func (s testSteps) settle(
	ctx context.Context, tx *database.Tx, test models.EmailAddressTest, end testEnd,
) (bool, error) {
	if end.findings == nil {
		end.findings = []TestFinding{}
	}

	row, err := models.New(tx).SettleEmailAddressTest(ctx, models.SettleEmailAddressTestParams{
		Status: end.status, InboundEmailID: end.inboundID, Findings: jsonOf(end.findings), LatencyMs: end.latency,
		Now: s.Clock.Now(), WorkspaceID: test.WorkspaceID, ID: test.ID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("settle email address test: %w", err)
	}

	return true, s.announce(ctx, tx, row)
}

/*
announce tells the workspace the test moved on.
*/
func (s testSteps) announce(ctx context.Context, tx *database.Tx, test models.EmailAddressTest) error {
	err := s.Events.Dispatch(ctx, tx, appevents.EmailAddressTestUpdated{
		Base: appevents.Base{
			ID: ids.New(), WorkspaceID: test.WorkspaceID, OccurredAt: s.Clock.Now(),
			Actor: appevents.Actor{Type: "system", ID: ids.Nil()},
		},
		AddressID: test.EmailAddressID, TestID: test.ID, Status: test.Status,
	})
	if err != nil {
		return fmt.Errorf("dispatch email address test: %w", err)
	}

	return nil
}

/*
latencySince returns the milliseconds from the start of the test to now,
held within what the column stores.
*/
func latencySince(test models.EmailAddressTest, now time.Time) int32 {
	ms := now.Sub(test.CreatedAt).Milliseconds()
	if ms < 0 {
		return 0
	}

	if ms > math.MaxInt32 {
		return math.MaxInt32
	}

	return int32(ms)
}

/*
ShowEmailAddressTest returns one forwarding test of an address.

Rules:
  - The caller needs email:manage.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type ShowEmailAddressTest struct {
	DB *database.DB
}

/*
Handle returns the test.
*/
func (a *ShowEmailAddressTest) Handle(
	ctx context.Context, m authz.Membership, addressID, testID ids.UUID,
) (AddressTest, error) {
	if !m.Can("email:manage") {
		return AddressTest{}, exceptions.ErrForbidden
	}

	var out AddressTest

	err := within(inWorkspace(ctx, m.Workspace.ID), a.DB, func(ctx context.Context, q *models.Queries) error {
		address, err := findAddress(ctx, q, m.Workspace.ID, addressID)
		if err != nil {
			return err
		}

		row, err := q.FindEmailAddressTest(ctx, models.FindEmailAddressTestParams{
			WorkspaceID: m.Workspace.ID, EmailAddressID: addressID, ID: testID,
		})
		if errors.Is(err, database.ErrNotFound) {
			return exceptions.ErrNotFound
		}

		if err != nil {
			return fmt.Errorf("find email address test: %w", err)
		}

		out = AddressTest{Test: row.EmailAddressTest, SentTo: address.EmailAddress.Address, MessageID: row.MessageID}

		return nil
	})

	return out, err
}

/*
CheckEmailAddressTest times out a forwarding test whose mail is not back
in time, with hints on what usually holds a forward up for the mail
service the address seems to use.
*/
type CheckEmailAddressTest struct {
	DB     *database.DB
	Clock  clock.Clock
	Events *events.Dispatcher
	Guides *RecommendGuide
}

/*
Handle times the test out unless it settled already or its time is not
up: a check run well before then, by a clock that runs ahead, leaves
the test waiting.
*/
func (a *CheckEmailAddressTest) Handle(ctx context.Context, workspaceID, testID ids.UUID) error {
	ctx = inWorkspace(ctx, workspaceID)
	steps := testSteps{Clock: a.Clock, Events: a.Events}

	var address models.EmailAddress

	test, err := a.find(ctx, workspaceID, testID, &address)
	if err != nil || test.Status == TestReceived || test.Status == TestFailed || test.Status == TestTimedOut {
		return err
	}

	if a.Clock.Now().Before(test.ExpiresAt.Add(-testEarlyRun)) {
		return nil
	}

	findings := timeoutHints(a.Guides.Handle(ctx, address.Address), address)

	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		_, err := steps.settle(ctx, tx, test, testEnd{status: TestTimedOut, findings: findings})

		return err
	})
}

/*
find reads the test and its address; a test or address gone since is
reported as settled, so nothing is done.
*/
func (a *CheckEmailAddressTest) find(
	ctx context.Context, workspaceID, testID ids.UUID, address *models.EmailAddress,
) (models.EmailAddressTest, error) {
	var test models.EmailAddressTest

	err := within(ctx, a.DB, func(ctx context.Context, q *models.Queries) error {
		var err error

		test, err = q.FindEmailAddressTestByID(ctx, models.FindEmailAddressTestByIDParams{
			WorkspaceID: workspaceID, ID: testID,
		})
		if err != nil {
			return err
		}

		*address, err = q.FindEmailAddressByID(ctx, models.FindEmailAddressByIDParams{
			WorkspaceID: workspaceID, ID: test.EmailAddressID,
		})

		return err
	})
	if errors.Is(err, database.ErrNotFound) {
		return models.EmailAddressTest{Status: TestTimedOut}, nil
	}

	if err != nil {
		return test, fmt.Errorf("find email address test: %w", err)
	}

	return test, nil
}

/*
PruneEmailAddressTests deletes forwarding tests older than thirty days,
in every workspace.
*/
type PruneEmailAddressTests struct {
	DB    *database.DB
	Clock clock.Clock
}

/*
Handle prunes the old tests and returns how many went.
*/
func (a *PruneEmailAddressTests) Handle(ctx context.Context) (int64, error) {
	pruned, err := models.New(a.DB).PruneEmailAddressTests(ctx, a.Clock.Now().Add(-testRetention))
	if err != nil {
		return 0, fmt.Errorf("prune email address tests: %w", err)
	}

	return pruned, nil
}
