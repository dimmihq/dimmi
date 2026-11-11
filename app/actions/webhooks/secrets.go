/*
|--------------------------------------------------------------------------
| Signing Secrets
|--------------------------------------------------------------------------
|
| Every endpoint has a secret that signs its requests, sealed with the
| workspace's key and shown only to those who manage webhooks; each
| reveal is audited. Rotating keeps the replaced secret signing for an
| overlap the admin picks, up to a week, so a receiver can deploy the new
| one without dropping a request; zero cuts the old one off at once,
| for a leaked secret. One rotation runs at a time unless forced, which
| drops the older secret immediately.
|
*/

package webhooks

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/webhooks"

	"github.com/dimmihq/dimmi/app/actions/audit"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Secrets is an endpoint's current secret and, while a rotation
overlaps, the previous one with when it stops signing.
*/
type Secrets struct {
	Current         string
	Previous        string
	PreviousExpires *time.Time
}

/*
RotationInput is a rotation of one endpoint's secret: how long the
replaced secret keeps signing, and whether to force it while an earlier
rotation still overlaps.
*/
type RotationInput struct {
	Membership authz.Membership
	EndpointID ids.UUID
	Overlap    time.Duration
	Force      bool
}

/*
secretAAD binds a sealed secret to the workspace it signs for.
*/
func secretAAD(ws ids.UUID) []byte {
	return []byte("webhook_endpoints.secret:" + ws.String())
}

/*
seal encrypts a secret with the workspace's key.
*/
func (w *Webhooks) seal(ctx context.Context, ws ids.UUID, secret webhooks.Secret) (encryption.Sealed, error) {
	sealed, err := encryption.Seal(ctx, w.Keys, encryption.Slot{
		Scope: encryption.Scope{Kind: "workspace", ID: ws.String()}, AAD: secretAAD(ws),
	}, secret.Encode())
	if err != nil {
		return nil, fmt.Errorf("seal webhook secret: %w", err)
	}

	return sealed, nil
}

/*
open decrypts a sealed secret.
*/
func (w *Webhooks) open(ctx context.Context, ws ids.UUID, sealed encryption.Sealed) (webhooks.Secret, error) {
	encoded, err := encryption.Open[string](ctx, w.Keys, sealed, secretAAD(ws))
	if err != nil {
		return webhooks.Secret{}, fmt.Errorf("open webhook secret: %w", err)
	}

	return webhooks.ParseSecret(encoded)
}

/*
signingSecrets returns every secret that signs the endpoint's requests
now: the current one, and the previous one while its overlap lasts.
*/
func (w *Webhooks) signingSecrets(ctx context.Context, e models.WebhookEndpoint) ([]webhooks.Secret, error) {
	current, err := w.open(ctx, e.WorkspaceID, e.SecretEncrypted)
	if err != nil {
		return nil, err
	}

	if len(e.PreviousSecretEncrypted) == 0 || !overlapping(e, w.Clock.Now()) {
		return []webhooks.Secret{current}, nil
	}

	previous, err := w.open(ctx, e.WorkspaceID, e.PreviousSecretEncrypted)
	if err != nil {
		return nil, err
	}

	return []webhooks.Secret{current, previous}, nil
}

/*
overlapping reports whether the endpoint's previous secret still signs.
*/
func overlapping(e models.WebhookEndpoint, now time.Time) bool {
	return e.PreviousSecretExpiresAt != nil && now.Before(*e.PreviousSecretExpiresAt)
}

/*
RevealSecret returns an endpoint's secrets and records who saw them.
*/
func (w *Webhooks) RevealSecret(ctx context.Context, m authz.Membership, id ids.UUID) (Secrets, error) {
	if err := requireWrite(m); err != nil {
		return Secrets{}, err
	}

	var out Secrets

	err := w.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, _ *database.Tx, q *models.Queries) error {
		e, err := findEndpoint(ctx, q, m.Workspace.ID, id)
		if err != nil {
			return err
		}

		if out, err = w.reveal(ctx, e); err != nil {
			return err
		}

		return w.record(ctx, m, e, audit.Entry{Event: "webhook_endpoint.secret_revealed"})
	})

	return out, err
}

/*
reveal opens the secrets of an endpoint for its manager.
*/
func (w *Webhooks) reveal(ctx context.Context, e models.WebhookEndpoint) (Secrets, error) {
	secrets, err := w.signingSecrets(ctx, e)
	if err != nil {
		return Secrets{}, err
	}

	out := Secrets{Current: secrets[0].Encode()}
	if len(secrets) > 1 {
		out.Previous, out.PreviousExpires = secrets[1].Encode(), e.PreviousSecretExpiresAt
	}

	return out, nil
}

/*
RotateSecret gives an endpoint a new secret and keeps the replaced one
signing for the overlap.
*/
func (w *Webhooks) RotateSecret(ctx context.Context, in RotationInput) (models.WebhookEndpoint, Secrets, error) {
	if err := requireWrite(in.Membership); err != nil {
		return models.WebhookEndpoint{}, Secrets{}, err
	}

	if in.Overlap < 0 || in.Overlap > w.Config.MaxRotationOverlap {
		return models.WebhookEndpoint{}, Secrets{}, exceptions.ErrWebhookRotationOverlap
	}

	var out models.WebhookEndpoint

	var secrets Secrets

	ws := in.Membership.Workspace.ID
	err := w.inWorkspace(ctx, ws, func(ctx context.Context, _ *database.Tx, q *models.Queries) error {
		e, err := q.LockWebhookEndpoint(ctx, models.LockWebhookEndpointParams{WorkspaceID: ws, ID: in.EndpointID})
		if err != nil {
			return notFound(err, "lock webhook endpoint")
		}

		if out, err = w.rotate(ctx, q, e, in); err != nil {
			return err
		}

		if secrets, err = w.reveal(ctx, out); err != nil {
			return err
		}

		return w.record(ctx, in.Membership, out, audit.Entry{
			Event:   "webhook_endpoint.secret_rotated",
			Changes: map[string]any{"overlap_seconds": int64(in.Overlap.Seconds())},
		})
	})

	return out, secrets, err
}

/*
rotate stores the new secret, keeping the current one as the previous
while the overlap lasts.
*/
func (w *Webhooks) rotate(
	ctx context.Context, q *models.Queries, e models.WebhookEndpoint, in RotationInput,
) (models.WebhookEndpoint, error) {
	now := w.Clock.Now()
	if overlapping(e, now) && !in.Force {
		return e, exceptions.ErrWebhookRotationInProgress
	}

	sealed, err := w.seal(ctx, e.WorkspaceID, webhooks.NewSecret())
	if err != nil {
		return e, err
	}

	params := models.SetWebhookEndpointSecretsParams{
		WorkspaceID: e.WorkspaceID, ID: e.ID, SecretEncrypted: sealed, Now: now,
	}
	if in.Overlap > 0 {
		expires := now.Add(in.Overlap)
		params.PreviousSecretEncrypted, params.PreviousSecretExpiresAt = e.SecretEncrypted, &expires
	}

	out, err := q.SetWebhookEndpointSecrets(ctx, params)
	if err != nil {
		return e, fmt.Errorf("rotate webhook secret: %w", err)
	}

	return out, nil
}

/*
ExpireSecrets forgets every previous secret whose overlap ended, in
every workspace with endpoints.
*/
func (w *Webhooks) ExpireSecrets(ctx context.Context) error {
	return w.eachWorkspace(ctx, func(ctx context.Context, ws ids.UUID, _ *database.Tx, q *models.Queries) error {
		_, err := q.ExpireWebhookEndpointSecrets(ctx, models.ExpireWebhookEndpointSecretsParams{
			WorkspaceID: ws, Now: w.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("expire webhook secrets: %w", err)
		}

		return nil
	})
}

/*
record writes the audit entry, its event and changes, about an
endpoint, naming only its host and the member who acted.
*/
func (w *Webhooks) record(ctx context.Context, m authz.Membership, e models.WebhookEndpoint, entry audit.Entry) error {
	if w.Audit == nil {
		return nil
	}

	entry.WorkspaceID, entry.ActorID = e.WorkspaceID, m.Member.ID
	entry.Subject = audit.Subject{Type: "webhook_endpoint", ID: e.ID, Label: Host(e.Url)}
	_, err := w.Audit.Handle(ctx, entry)

	return err
}
