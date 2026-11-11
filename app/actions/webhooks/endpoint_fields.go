/*
|--------------------------------------------------------------------------
| Endpoint Fields
|--------------------------------------------------------------------------
|
| What an endpoint may be set to. It subscribes to one to a hundred
| catalog entries, each an exact type, an area wildcard or everything;
| duplicates collapse and everything absorbs the rest. Its inbox filter
| names only the workspace's own inboxes. Metadata is a few short
| labels for the developer's own use. The api_version is one this
| server speaks.
|
*/

package webhooks

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Limits of an endpoint's settings.
*/
const (
	maxEventTypes    = 100
	maxDescription   = 255
	maxMetadataKeys  = 20
	maxMetadataValue = 500
)

/*
EndpointInput is an endpoint to create, or the fields of one to change:
nil leaves a field as it is. ClearInboxes removes the inbox filter.
*/
type EndpointInput struct {
	Membership   authz.Membership
	URL          *string
	Description  *string
	EventTypes   *[]string
	IncludeNotes *bool
	InboxIDs     *[]ids.UUID
	ClearInboxes bool
	APIVersion   *string
	Metadata     *map[string]string
	Enabled      *bool
}

/*
apply sets the input's fields on the endpoint, checked.
*/
func (w *Webhooks) apply(
	ctx context.Context, q *models.Queries, e models.WebhookEndpoint, in EndpointInput,
) (models.WebhookEndpoint, error) {
	if in.URL != nil {
		normal, err := w.NormalizeURL(ctx, *in.URL)
		if err != nil {
			return e, err
		}

		e.Url = normal
	}

	if in.Description != nil {
		e.Description = strings.TrimSpace(*in.Description)
		if len([]rune(e.Description)) > maxDescription {
			return e, exceptions.ErrWebhookMetadataInvalid.OnField("description")
		}
	}

	if in.EventTypes != nil {
		types, err := normalizeEventTypes(*in.EventTypes)
		if err != nil {
			return e, err
		}

		e.EventTypes = types
	}

	if in.IncludeNotes != nil {
		e.IncludeNotes = *in.IncludeNotes
	}

	if err := applyVersionAndMetadata(&e, in); err != nil {
		return e, err
	}

	err := applyInboxes(ctx, q, &e, in)

	return e, err
}

/*
applyVersionAndMetadata sets the payload version and the metadata.
*/
func applyVersionAndMetadata(e *models.WebhookEndpoint, in EndpointInput) error {
	if in.APIVersion != nil {
		if *in.APIVersion != APIVersion {
			return exceptions.ErrWebhookAPIVersionUnsupported
		}

		e.ApiVersion = *in.APIVersion
	}

	if in.Metadata == nil {
		return nil
	}

	if len(*in.Metadata) > maxMetadataKeys {
		return exceptions.ErrWebhookMetadataInvalid
	}

	for key, value := range *in.Metadata {
		if key == "" || len([]rune(key)) > maxMetadataValue || len([]rune(value)) > maxMetadataValue {
			return exceptions.ErrWebhookMetadataInvalid
		}
	}

	e.Metadata, _ = json.Marshal(*in.Metadata)

	return nil
}

/*
applyInboxes sets the inbox filter, checking every inbox is the
workspace's own.
*/
func applyInboxes(ctx context.Context, q *models.Queries, e *models.WebhookEndpoint, in EndpointInput) error {
	if in.ClearInboxes {
		e.InboxIds = nil

		return nil
	}

	if in.InboxIDs == nil {
		return nil
	}

	unique := slices.Compact(slices.SortedFunc(slices.Values(*in.InboxIDs), compareIDs))
	for _, id := range unique {
		_, err := q.FindInbox(ctx, models.FindInboxParams{WorkspaceID: e.WorkspaceID, ID: id})
		if err != nil {
			return notFoundAs(err, exceptions.ErrWebhookInboxUnknown, "find webhook inbox")
		}
	}

	e.InboxIds = unique

	return nil
}

/*
compareIDs orders ids by their text.
*/
func compareIDs(a, b ids.UUID) int {
	return strings.Compare(a.String(), b.String())
}

/*
normalizeEventTypes checks a subscription and returns it sorted without
duplicates; with * in it, * alone.
*/
func normalizeEventTypes(types []string) ([]string, error) {
	out := make([]string, 0, len(types))
	for _, t := range types {
		t = strings.TrimSpace(t)
		if !appevents.ValidWebhookSubscription(t) {
			return nil, exceptions.ErrUnknownWebhookEventType.With("event_type", t)
		}

		out = append(out, t)
	}

	if len(out) == 0 {
		return nil, exceptions.ErrWebhookEventTypesRequired
	}

	if slices.Contains(out, appevents.WebhookWildcard) {
		return []string{appevents.WebhookWildcard}, nil
	}

	out = slices.Compact(slices.Sorted(slices.Values(out)))
	if len(out) > maxEventTypes {
		return nil, exceptions.ErrUnknownWebhookEventType.With("max", maxEventTypes)
	}

	return out, nil
}

/*
emptyMetadata is the metadata of an endpoint created without any.
*/
func emptyMetadata() json.RawMessage {
	return json.RawMessage(`{}`)
}

/*
wrapf wraps an error with what was being attempted.
*/
func wrapf(what string, err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("%s: %w", what, err)
}
