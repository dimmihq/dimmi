/*
|--------------------------------------------------------------------------
| Endpoints
|--------------------------------------------------------------------------
|
| Listing, adding, changing and deleting endpoints. A new endpoint gets
| its secret, shown once in the answer, and every owner is mailed which
| host now receives the workspace's data and who added it; so is turning
| on internal notes. A workspace holds as many endpoints as its plan
| allows. Turning an endpoint off cancels what waits to be sent;
| turning it back on resets its health but resends nothing: missed
| deliveries are redelivered from the log. Deleting removes the log
| with it.
|
*/

package webhooks

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/webhooks"

	"github.com/dimmihq/dimmi/app/actions/audit"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/entitlements"
)

/*
urlConstraint keeps a workspace's endpoint URLs apart.
*/
const urlConstraint = "webhook_endpoints_workspace_id_url_key"

/*
List returns the workspace's endpoints.
*/
func (w *Webhooks) List(ctx context.Context, m authz.Membership) ([]models.WebhookEndpoint, error) {
	if err := requireRead(m); err != nil {
		return nil, err
	}

	var out []models.WebhookEndpoint

	err := w.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, _ *database.Tx, q *models.Queries) error {
		var err error
		out, err = q.ListWebhookEndpoints(ctx, m.Workspace.ID)

		return wrapf("list webhook endpoints", err)
	})

	return out, err
}

/*
Show returns one endpoint.
*/
func (w *Webhooks) Show(ctx context.Context, m authz.Membership, id ids.UUID) (models.WebhookEndpoint, error) {
	if err := requireRead(m); err != nil {
		return models.WebhookEndpoint{}, err
	}

	var out models.WebhookEndpoint

	err := w.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, _ *database.Tx, q *models.Queries) error {
		var err error
		out, err = findEndpoint(ctx, q, m.Workspace.ID, id)

		return err
	})

	return out, err
}

/*
Create adds an endpoint and returns it with its secret.
*/
func (w *Webhooks) Create(ctx context.Context, in EndpointInput) (models.WebhookEndpoint, string, error) {
	if err := requireWrite(in.Membership); err != nil {
		return models.WebhookEndpoint{}, "", err
	}

	if in.URL == nil {
		return models.WebhookEndpoint{}, "", exceptions.ErrInvalidWebhookURL
	}

	if in.EventTypes == nil {
		return models.WebhookEndpoint{}, "", exceptions.ErrWebhookEventTypesRequired
	}

	var out models.WebhookEndpoint

	secret := webhooks.NewSecret()
	ws := in.Membership.Workspace.ID

	err := w.inWorkspace(ctx, ws, func(ctx context.Context, tx *database.Tx, q *models.Queries) error {
		fields, err := w.apply(ctx, q, models.WebhookEndpoint{
			WorkspaceID: ws, ApiVersion: APIVersion, Metadata: emptyMetadata(),
			CreatedByMemberID: &in.Membership.Member.ID,
		}, in)
		if err != nil {
			return err
		}

		if out, err = w.insert(ctx, q, fields, secret); err != nil {
			return err
		}

		if err := w.record(ctx, in.Membership, out, audit.Entry{
			Event: "webhook_endpoint.created", Changes: endpointChanges(out),
		}); err != nil {
			return err
		}

		return w.mailAdded(ctx, tx, in.Membership, out)
	})

	return out, secret.Encode(), err
}

/*
insert stores the endpoint, with the member who created it, once the
workspace's cap allows another.
*/
func (w *Webhooks) insert(
	ctx context.Context, q *models.Queries, e models.WebhookEndpoint, secret webhooks.Secret,
) (models.WebhookEndpoint, error) {
	if err := w.checkLimit(ctx, q, e.WorkspaceID); err != nil {
		return e, err
	}

	sealed, err := w.seal(ctx, e.WorkspaceID, secret)
	if err != nil {
		return e, err
	}

	out, err := q.CreateWebhookEndpoint(ctx, models.CreateWebhookEndpointParams{
		WorkspaceID: e.WorkspaceID, Url: e.Url, Description: e.Description, EventTypes: e.EventTypes,
		IncludeNotes: e.IncludeNotes, InboxIds: e.InboxIds, ApiVersion: e.ApiVersion, Metadata: e.Metadata,
		SecretEncrypted: sealed, CreatedByMemberID: e.CreatedByMemberID, Now: w.Clock.Now(),
	})
	if database.IsUniqueViolation(err, urlConstraint) {
		return out, exceptions.ErrWebhookURLTaken
	}

	return out, wrapf("create webhook endpoint", err)
}

/*
checkLimit counts the workspace's endpoints under a lock on it.
*/
func (w *Webhooks) checkLimit(ctx context.Context, q *models.Queries, ws ids.UUID) error {
	if _, err := q.LockWorkspace(ctx, ws); err != nil {
		return fmt.Errorf("lock workspace: %w", err)
	}

	limit, err := w.Entitlements.Limit(ctx, ws, entitlements.KeyWebhookEndpoints)
	if err != nil {
		return fmt.Errorf("read webhook endpoint limit: %w", err)
	}

	used, err := q.CountWebhookEndpoints(ctx, ws)
	if err != nil {
		return fmt.Errorf("count webhook endpoints: %w", err)
	}

	if limit != entitlements.Unlimited && used >= limit {
		return exceptions.ErrWebhookEndpointLimitReached.
			With("key", entitlements.KeyWebhookEndpoints).With("limit", limit)
	}

	return nil
}

/*
Update changes an endpoint's settings and returns it as it now is.
*/
func (w *Webhooks) Update(ctx context.Context, id ids.UUID, in EndpointInput) (models.WebhookEndpoint, error) {
	if err := requireWrite(in.Membership); err != nil {
		return models.WebhookEndpoint{}, err
	}

	var out models.WebhookEndpoint

	ws := in.Membership.Workspace.ID
	err := w.inWorkspace(ctx, ws, func(ctx context.Context, tx *database.Tx, q *models.Queries) error {
		current, err := q.LockWebhookEndpoint(ctx, models.LockWebhookEndpointParams{WorkspaceID: ws, ID: id})
		if err != nil {
			return notFound(err, "lock webhook endpoint")
		}

		changed, err := w.apply(ctx, q, current, in)
		if err != nil {
			return err
		}

		if out, err = w.save(ctx, q, current, w.toggle(changed, current, in)); err != nil {
			return err
		}

		if err := w.record(ctx, in.Membership, out, audit.Entry{
			Event: "webhook_endpoint.updated", Changes: diff(current, out),
		}); err != nil {
			return err
		}

		if out.IncludeNotes && !current.IncludeNotes {
			return w.mailAdded(ctx, tx, in.Membership, out)
		}

		return nil
	})

	return out, err
}

/*
toggle applies an enable or disable, and resets the health of an
endpoint turned on or moved to a new URL.
*/
func (w *Webhooks) toggle(e, before models.WebhookEndpoint, in EndpointInput) models.WebhookEndpoint {
	now := w.Clock.Now()
	if in.Enabled != nil && *in.Enabled != before.Enabled {
		e.Enabled = *in.Enabled
		e.DisabledReason, e.DisabledAt = nil, nil

		if !e.Enabled {
			reason := "manual"
			e.DisabledReason, e.DisabledAt = &reason, &now
		}
	}

	if (e.Enabled && !before.Enabled) || e.Url != before.Url {
		e.Health, e.FailingSince, e.FailingNotifiedAt = "healthy", nil, nil
	}

	return e
}

/*
save stores the endpoint's settings and cancels what it may no longer
send: everything when it was turned off, notes when they were.
*/
func (w *Webhooks) save(
	ctx context.Context, q *models.Queries, before, e models.WebhookEndpoint,
) (models.WebhookEndpoint, error) {
	now := w.Clock.Now()

	out, err := q.UpdateWebhookEndpoint(ctx, models.UpdateWebhookEndpointParams{
		WorkspaceID: e.WorkspaceID, ID: e.ID, Url: e.Url, Description: e.Description, EventTypes: e.EventTypes,
		IncludeNotes: e.IncludeNotes, InboxIds: e.InboxIds, ApiVersion: e.ApiVersion, Metadata: e.Metadata,
		Enabled: e.Enabled, DisabledReason: e.DisabledReason, DisabledAt: e.DisabledAt, Health: e.Health,
		FailingSince: e.FailingSince, FailingNotifiedAt: e.FailingNotifiedAt, Now: now,
	})
	if database.IsUniqueViolation(err, urlConstraint) {
		return out, exceptions.ErrWebhookURLTaken
	}

	if err != nil {
		return out, fmt.Errorf("update webhook endpoint: %w", err)
	}

	if before.Enabled && !out.Enabled {
		_, err = q.CancelWebhookDeliveries(ctx, models.CancelWebhookDeliveriesParams{
			WorkspaceID: out.WorkspaceID, EndpointID: out.ID, Now: now,
		})
	}

	if err == nil && before.IncludeNotes && !out.IncludeNotes {
		_, err = q.CancelWebhookNoteDeliveries(ctx, models.CancelWebhookNoteDeliveriesParams{
			WorkspaceID: out.WorkspaceID, EndpointID: out.ID, Now: now,
		})
	}

	return out, wrapf("cancel webhook deliveries", err)
}

/*
Delete removes an endpoint with its delivery log.
*/
func (w *Webhooks) Delete(ctx context.Context, m authz.Membership, id ids.UUID) error {
	if err := requireWrite(m); err != nil {
		return err
	}

	return w.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, _ *database.Tx, q *models.Queries) error {
		e, err := findEndpoint(ctx, q, m.Workspace.ID, id)
		if err != nil {
			return err
		}

		if _, err := q.DeleteWebhookEndpoint(ctx, models.DeleteWebhookEndpointParams{
			WorkspaceID: e.WorkspaceID, ID: e.ID,
		}); err != nil {
			return fmt.Errorf("delete webhook endpoint: %w", err)
		}

		return w.record(ctx, m, e, audit.Entry{Event: "webhook_endpoint.deleted"})
	})
}

/*
endpointChanges is what the audit log keeps of a new endpoint.
*/
func endpointChanges(e models.WebhookEndpoint) map[string]any {
	return map[string]any{"host": Host(e.Url), "event_types": e.EventTypes, "include_notes": e.IncludeNotes}
}

/*
diff is what the audit log keeps of a change to an endpoint: the
settings that changed, the URL by host only.
*/
func diff(before, after models.WebhookEndpoint) map[string]any {
	out := map[string]any{}
	if before.Url != after.Url {
		out["host"] = map[string]any{"from": Host(before.Url), "to": Host(after.Url)}
	}

	if before.IncludeNotes != after.IncludeNotes {
		out["include_notes"] = map[string]any{"from": before.IncludeNotes, "to": after.IncludeNotes}
	}

	if before.Enabled != after.Enabled {
		out["enabled"] = map[string]any{"from": before.Enabled, "to": after.Enabled}
	}

	if fmt.Sprint(before.EventTypes) != fmt.Sprint(after.EventTypes) {
		out["event_types"] = map[string]any{"from": before.EventTypes, "to": after.EventTypes}
	}

	return out
}
