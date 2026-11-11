/*
|--------------------------------------------------------------------------
| Sending A Delivery
|--------------------------------------------------------------------------
|
| One attempt posts the stored bytes, signed afresh with a new
| timestamp, to the URL the endpoint has now. Redirects are never
| followed, a slow receiver gets the configured time and no more, and
| only the start of the answer is read. A 2xx is delivered; 410 Gone
| turns the endpoint off; anything else, and every failure on the way,
| is retried on the schedule. Each attempt leaves one row in the log
| with the headers sent and the start of what came back. A key store
| that cannot open the secret postpones the attempt without using it.
|
*/

package webhooks

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/webhooks"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Limits of one attempt: the requests per second an endpoint receives,
how long a sealed secret may stay unopenable before attempts wait, and
how many response headers the log keeps and how long each.
*/
const (
	perEndpointPerSecond = 25
	keyOutageWait        = 30 * time.Second
	maxLoggedHeaders     = 30
	maxHeaderValue       = 512
	maxErrorMessage      = 500
	userAgent            = "Dimmi-Webhooks/1.0 (+https://trydimmi.com/docs/webhooks)"
)

/*
Error codes an attempt records beside the network failure classes.
*/
const (
	errorHTTP     = "http_error"
	errorRedirect = "redirect_not_followed"
)

/*
Deliver sends one delivery once, as the job asked.
*/
func (w *Webhooks) Deliver(ctx context.Context, args jobs.DeliverWebhookArgs) error {
	_, _, err := w.attempt(ctx, args.WorkspaceID, args.DeliveryID, args.Trigger)

	return err
}

/*
paced reports whether the delivery's endpoint may receive another
request this second. Without the counter, requests go out unpaced.
*/
func (w *Webhooks) paced(ctx context.Context, endpoint ids.UUID) bool {
	if w.Limiter == nil {
		return true
	}

	res, err := w.Limiter.Attempt(ctx, "webhooks:endpoint:"+endpoint.String(), perEndpointPerSecond, time.Second)

	return err != nil || res.Allowed
}

/*
target is a delivery with the endpoint it goes to.
*/
type target struct {
	delivery models.WebhookDelivery
	endpoint models.WebhookEndpoint
}

/*
attempt sends the delivery and records the outcome, returning the
delivery as it now is and the attempt. A delivery that is gone, done,
or whose endpoint was turned off sends nothing.
*/
func (w *Webhooks) attempt(
	ctx context.Context, ws, id ids.UUID, trigger string,
) (models.WebhookDelivery, models.WebhookDeliveryAttempt, error) {
	t, ok, err := w.load(ctx, ws, id, trigger)
	if err != nil || !ok {
		return t.delivery, models.WebhookDeliveryAttempt{}, err
	}

	if trigger == triggerAuto && !w.paced(ctx, t.endpoint.ID) {
		return t.delivery, models.WebhookDeliveryAttempt{}, queue.Postpone(time.Second)
	}

	secrets, err := w.signingSecrets(ctx, t.endpoint)
	if err != nil {
		return t.delivery, models.WebhookDeliveryAttempt{}, queue.Postpone(keyOutageWait)
	}

	result := w.send(ctx, t, secrets, trigger)

	return w.recordOutcome(ctx, t, result)
}

/*
load reads a delivery and its endpoint, reporting false when there is
nothing to send.
*/
func (w *Webhooks) load(ctx context.Context, ws, id ids.UUID, trigger string) (target, bool, error) {
	var t target

	err := w.inWorkspace(ctx, ws, func(ctx context.Context, _ *database.Tx, q *models.Queries) error {
		var err error

		t.delivery, err = q.FindWebhookDelivery(ctx, models.FindWebhookDeliveryParams{WorkspaceID: ws, ID: id})
		if err != nil {
			return err
		}

		t.endpoint, err = findEndpoint(ctx, q, ws, t.delivery.EndpointID)

		return err
	})
	if errors.Is(err, database.ErrNotFound) || errors.Is(err, exceptions.ErrNotFound) {
		return t, false, nil
	}

	if err != nil {
		return t, false, fmt.Errorf("load webhook delivery: %w", err)
	}

	if trigger != triggerAuto {
		return t, true, nil
	}

	waiting := t.delivery.Status == statusPending || t.delivery.Status == statusRetrying

	return t, waiting && t.endpoint.Enabled, nil
}

/*
sent is what one request brought back, with the trigger that sent it.
*/
type sent struct {
	trigger    string
	at         time.Time
	headers    map[string]string
	status     int
	respHeader map[string]string
	body       string
	errorCode  string
	errorText  string
	duration   time.Duration
	retryAfter time.Duration
}

/*
send posts the delivery's bytes, signed now, and reads the answer.
*/
func (w *Webhooks) send(ctx context.Context, t target, secrets []webhooks.Secret, trigger string) sent {
	now := w.Clock.Now()
	number := strconv.Itoa(int(t.delivery.AttemptCount) + 1)
	eventID := models.PublicID(models.PrefixWebhookEvent, t.delivery.EventID)
	headers := map[string]string{
		"content-type":           "application/json",
		"user-agent":             userAgent,
		webhooks.HeaderID:        eventID,
		webhooks.HeaderTimestamp: strconv.FormatInt(now.Unix(), 10),
		webhooks.HeaderSignature: webhooks.SignAll(secrets, eventID, now, t.delivery.Payload),
		"dimmi-event-type":       t.delivery.EventType,
		"dimmi-delivery-id":      models.PublicID(models.PrefixWebhookDelivery, t.delivery.ID),
		"dimmi-attempt":          number,
		"dimmi-api-version":      t.endpoint.ApiVersion,
		"dimmi-endpoint-id":      EndpointPublicID(t.endpoint.ID),
	}

	if trigger == triggerTest {
		headers["dimmi-test"] = "true"
	}

	req := w.HTTP.New().WithoutRedirects().Stream().Timeout(w.Config.Timeout)
	for name, value := range headers {
		req = req.WithHeader(name, value)
	}

	started := clock.System().Now()
	resp, err := req.Post(ctx, t.endpoint.Url, t.delivery.Payload)
	out := sent{trigger: trigger, at: now, headers: headers}

	if err != nil {
		out.errorCode, out.errorText = httpclient.Classify(err), cut(err.Error(), maxErrorMessage)
		out.duration = clock.System().Since(started)

		return out
	}

	defer func() { _ = resp.Close() }()

	w.read(resp, &out)
	out.duration = clock.System().Since(started)

	return out
}

/*
read reads the start of an answer and names its class.
*/
func (w *Webhooks) read(resp *httpclient.Response, out *sent) {
	body, err := io.ReadAll(io.LimitReader(resp.Reader(), w.Config.MaxResponseBytes))
	out.status, out.respHeader = resp.Status, loggedHeaders(resp.Header)
	out.body = cleanBody(body, w.Config.StoredResponseBytes)
	out.retryAfter = parseRetryAfter(resp.Header.Get("Retry-After"), out.at)

	switch {
	case success(resp.Status):
		return
	case resp.Status >= 300 && resp.Status < 400:
		out.errorCode = errorRedirect
		location := resp.Header.Get("Location")
		out.errorText = cut("Redirects are not followed. Update the URL to "+location, maxErrorMessage)
	case err != nil:
		out.errorCode = httpclient.Classify(err)
	default:
		out.errorCode = errorHTTP
	}
}

/*
success reports a 2xx status.
*/
func success(status int) bool {
	return status >= 200 && status < 300
}

/*
loggedHeaders keeps the first response headers, values cut short and
credentials hidden.
*/
func loggedHeaders(h map[string][]string) map[string]string {
	out := map[string]string{}
	for name, values := range h {
		if len(out) >= maxLoggedHeaders {
			break
		}

		value := cut(strings.Join(values, ", "), maxHeaderValue)
		switch strings.ToLower(name) {
		case "set-cookie", "authorization", "cookie":
			value = "[redacted]"
		}

		out[strings.ToLower(name)] = value
	}

	return out
}

/*
cleanBody keeps the start of a body as valid text without NUL bytes.
*/
func cleanBody(body []byte, limit int) string {
	text := strings.ToValidUTF8(string(body), "�")
	text = strings.ReplaceAll(text, "\x00", "")

	return cut(text, limit)
}

/*
cut shortens text to at most n bytes without splitting a character.
*/
func cut(text string, n int) string {
	if len(text) <= n {
		return text
	}

	text = text[:n]
	for !utf8.ValidString(text) {
		text = text[:len(text)-1]
	}

	return text
}
