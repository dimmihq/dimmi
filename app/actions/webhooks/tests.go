/*
|--------------------------------------------------------------------------
| Test Sends
|--------------------------------------------------------------------------
|
| A developer checks their receiver by sending it a test: one real,
| signed request, answered while they wait, and stored in the log like
| any delivery but marked as a test. Its payload is a sample, never the
| workspace's data, so it can be sent to an endpoint that is turned off,
| and it never changes the endpoint's health. A receiver that refuses
| the signature gets a hint that the raw body must be verified. Tests
| are capped per endpoint and minute.
|
*/

package webhooks

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/contracts"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
TestInput is a test send to one endpoint, of the event type, the test
type when empty.
*/
type TestInput struct {
	Membership authz.Membership
	EndpointID ids.UUID
	Type       string
}

/*
TestResult is a test's delivery and its one attempt.
*/
type TestResult struct {
	Delivery models.WebhookDelivery
	Attempt  models.WebhookDeliveryAttempt
}

/*
Test sends a sample event to the endpoint and returns how it went.
*/
func (w *Webhooks) Test(ctx context.Context, in TestInput) (TestResult, error) {
	if err := requireWrite(in.Membership); err != nil {
		return TestResult{}, err
	}

	kind, err := testKind(in.Type)
	if err != nil {
		return TestResult{}, err
	}

	in.Type = kind

	if err := w.throttleTest(ctx, in.EndpointID); err != nil {
		return TestResult{}, err
	}

	ws := in.Membership.Workspace.ID

	var row models.WebhookDelivery

	err = w.inWorkspace(ctx, ws, func(ctx context.Context, _ *database.Tx, q *models.Queries) error {
		e, err := findEndpoint(ctx, q, ws, in.EndpointID)
		if err != nil {
			return err
		}

		row, err = w.storeTest(ctx, q, e, in)

		return err
	})
	if err != nil {
		return TestResult{}, err
	}

	d, a, err := w.attempt(ctx, ws, row.ID, triggerTest)

	return TestResult{Delivery: d, Attempt: a}, err
}

/*
testKind returns the event type a test sends, refusing types the
catalog does not list.
*/
func testKind(kind string) (string, error) {
	if kind == "" || kind == appevents.WebhookTestType {
		return appevents.WebhookTestType, nil
	}

	if _, ok := appevents.FindWebhookEventType(kind); !ok {
		return "", exceptions.ErrWebhookTestTypeUnknown
	}

	return kind, nil
}

/*
throttleTest caps test sends per endpoint and minute.
*/
func (w *Webhooks) throttleTest(ctx context.Context, endpoint ids.UUID) error {
	if w.Limiter == nil {
		return nil
	}

	res, err := w.Limiter.Attempt(ctx, "webhooks:test:"+endpoint.String(), max(w.Config.TestsPerMinute, 1), time.Minute)
	if err == nil && !res.Allowed {
		return exceptions.ErrWebhookTestThrottled
	}

	return nil
}

/*
storeTest stores the test delivery of the input's event type with its
sample payload.
*/
func (w *Webhooks) storeTest(
	ctx context.Context, q *models.Queries, e models.WebhookEndpoint, in TestInput,
) (models.WebhookDelivery, error) {
	m, kind := in.Membership, in.Type
	actor := m.Member.ID
	args := jobs.FanOutWebhookEventArgs{
		WorkspaceID: e.WorkspaceID, EventID: ids.New(), Type: kind, OccurredAt: w.Clock.Now(),
		ActorType: "member", ActorID: &actor,
	}

	env := envelopeFor(args, 0, m.Workspace.TestMode)
	env.Test = true

	payload, err := w.encode(env, nil, sampleObject(kind), reference{kind: subjectConversation, id: ids.New()})
	if err != nil {
		return models.WebhookDelivery{}, err
	}

	row, err := q.CreateWebhookDelivery(ctx, models.CreateWebhookDeliveryParams{
		WorkspaceID: e.WorkspaceID, EndpointID: e.ID, EventID: args.EventID, EventType: kind,
		Payload: payload.body, PayloadBytes: clampInt32(int64(len(payload.body))), Test: true, Now: w.Clock.Now(),
	})

	return row, wrapf("store webhook test", err)
}

/*
sampleObject is the object a test carries: a greeting for the test
type, else a made-up object of the type's kind with a fresh id that
names nothing in the workspace.
*/
func sampleObject(kind string) map[string]any {
	entry, ok := appevents.FindWebhookEventType(kind)
	if !ok {
		return map[string]any{"object": "webhook_test", "message": "Hello from dimmi"}
	}

	ref := reference{kind: entry.Object, id: ids.New()}

	return map[string]any{"id": ref.publicRef(), "object": entry.Object, "sample": true}
}

/*
VerifyHint is the advice a test answered 401 or 403 carries, empty for
any other answer.
*/
func VerifyHint(status *int32) string {
	if status == nil || (*status != 401 && *status != 403) {
		return ""
	}

	return "Verify the raw body bytes: re-encoding the JSON changes the signature."
}

/*
held reports whether the retention guard holds the workspace's log.
*/
func (w *Webhooks) held(ctx context.Context, ws ids.UUID) (bool, error) {
	if w.Retention == nil {
		return false, nil
	}

	ok, err := w.Retention.MayDestroy(ctx, ws, retentionSubject(ws))
	if err != nil {
		return true, wrapf("ask retention guard", err)
	}

	return !ok, nil
}

/*
ContactEraser deletes the stored payloads about a contact being erased.
*/
type ContactEraser struct {
	Webhooks *Webhooks
}

/*
Name says what the eraser removes.
*/
func (ContactEraser) Name() string {
	return "webhook_deliveries"
}

/*
Erase deletes the contact's stored payloads.
*/
func (e ContactEraser) Erase(ctx context.Context, ws, contact ids.UUID) error {
	return e.Webhooks.EraseContact(ctx, ws, contact)
}

/*
retentionSubject is what the guard is asked before a workspace's log
is pruned.
*/
func retentionSubject(ws ids.UUID) contracts.RetentionSubject {
	return contracts.RetentionSubject{Kind: contracts.RetentionWorkspace, ID: ws}
}
