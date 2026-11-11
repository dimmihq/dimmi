/*
|--------------------------------------------------------------------------
| Payloads
|--------------------------------------------------------------------------
|
| Every request carries one envelope: the event's id, type and time,
| the workspace, an ordering number for its object, who caused it, and
| the object as the REST API shows it at fan-out, beside what the event
| changed. The envelope is encoded once and those exact bytes are
| stored, signed and resent on every attempt. An envelope over the size
| cap replaces the object with a reference to fetch it by, and drops
| large extras, so an event is never dropped for its size. An object
| that is gone shows as deleted.
|
*/

package webhooks

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxThinExtras is the largest encoded extras a reference payload keeps.
*/
const maxThinExtras = 16 << 10

/*
Envelope is the body of every webhook request.
*/
type Envelope struct {
	ID          string         `json:"id"`
	Object      string         `json:"object"`
	Type        string         `json:"type"`
	APIVersion  string         `json:"api_version"`
	CreatedAt   time.Time      `json:"created_at"`
	WorkspaceID string         `json:"workspace_id"`
	Sequence    int64          `json:"sequence"`
	Test        bool           `json:"test"`
	TestMode    bool           `json:"test_mode"`
	Truncated   bool           `json:"truncated"`
	Actor       EnvelopeActor  `json:"actor"`
	Data        map[string]any `json:"data"`
}

/*
EnvelopeActor is who caused an event: member, contact, agent, system
or integration, with their public id, null for the system.
*/
type EnvelopeActor struct {
	Type string  `json:"type"`
	ID   *string `json:"id"`
}

/*
encoded is a payload ready to store: its bytes and whether the object
was replaced by a reference.
*/
type encoded struct {
	body []byte
	thin bool
}

/*
actorPrefixes maps an actor type to the prefix of its public id.
*/
func actorPrefix(kind string) (models.Prefix, bool) {
	switch kind {
	case "member":
		return models.PrefixMember, true
	case "contact":
		return models.PrefixContact, true
	}

	return "", false
}

/*
envelopeFor starts the envelope of an occurrence.
*/
func envelopeFor(args jobs.FanOutWebhookEventArgs, sequence int64, testMode bool) Envelope {
	actor := EnvelopeActor{Type: args.ActorType}
	if prefix, ok := actorPrefix(args.ActorType); ok && args.ActorID != nil {
		id := models.PublicID(prefix, *args.ActorID)
		actor.ID = &id
	}

	return Envelope{
		ID: models.PublicID(models.PrefixWebhookEvent, args.EventID), Object: "event", Type: args.Type,
		APIVersion: APIVersion, CreatedAt: args.OccurredAt.UTC(),
		WorkspaceID: models.PublicID(models.PrefixWorkspace, args.WorkspaceID), Sequence: sequence,
		TestMode: testMode, Actor: actor, Data: map[string]any{},
	}
}

/*
encode writes the envelope with the object, falling back to a reference
when the whole is over the cap.
*/
func (w *Webhooks) encode(env Envelope, extras json.RawMessage, object any, ref reference) (encoded, error) {
	data := map[string]any{}
	if len(extras) > 0 {
		if err := json.Unmarshal(extras, &data); err != nil {
			return encoded{}, fmt.Errorf("read webhook extras: %w", err)
		}
	}

	data["object"] = object
	env.Data = data

	body, err := json.Marshal(env)
	if err != nil {
		return encoded{}, fmt.Errorf("encode webhook payload: %w", err)
	}

	if len(body) <= w.Config.MaxPayloadBytes {
		return encoded{body: body}, nil
	}

	thin := map[string]any{}
	if len(extras) <= maxThinExtras {
		_ = json.Unmarshal(extras, &thin)
	}

	thin["object"] = w.referenceObject(ref)
	env.Data, env.Truncated = thin, true

	body, err = json.Marshal(env)

	return encoded{body: body, thin: true}, wrapf("encode thin webhook payload", err)
}

/*
reference names the object a payload is about.
*/
type reference struct {
	kind string
	id   ids.UUID
}

/*
prefixOf returns the public id prefix of a subject.
*/
func prefixOf(kind string) models.Prefix {
	switch kind {
	case subjectMessage:
		return models.PrefixMessage
	case subjectContact:
		return models.PrefixContact
	case subjectTag:
		return models.PrefixTag
	}

	return models.PrefixConversation
}

/*
publicRef writes the reference's public id.
*/
func (r reference) publicRef() string {
	if r.kind == subjectContactEvent {
		return models.PublicID(models.PrefixWebhookEvent, r.id)
	}

	return models.PublicID(prefixOf(r.kind), r.id)
}

/*
referenceObject is what replaces an object too large to send: its id,
kind and where the REST API serves it.
*/
func (w *Webhooks) referenceObject(r reference) map[string]any {
	out := map[string]any{"id": r.publicRef(), "object": r.kind}
	if path := apiPath(r.kind); path != "" {
		self := strings.TrimRight(w.AppURL, "/") + "/api/v1/" + path + "/" + r.publicRef()
		out["links"] = map[string]any{"self": self}
	}

	return out
}

/*
apiPath is the REST collection that serves objects of the kind.
*/
func apiPath(kind string) string {
	switch kind {
	case subjectConversation:
		return "conversations"
	case subjectContact:
		return "contacts"
	case subjectTag:
		return "tags"
	}

	return ""
}

/*
tombstone is what a payload shows of an object that no longer exists.
*/
func tombstone(r reference) map[string]any {
	return map[string]any{"id": r.publicRef(), "object": r.kind, "deleted": true}
}

/*
contactEventObject is the object a tracked event carries: the event
and the contact it was tracked for.
*/
func contactEventObject(args jobs.FanOutWebhookEventArgs) map[string]any {
	out := map[string]any{
		"id": models.PublicID(models.PrefixWebhookEvent, args.EventID), "object": subjectContactEvent,
		"contact":     map[string]any{"id": models.PublicID(models.PrefixContact, args.SubjectID), "object": "contact"},
		"occurred_at": args.OccurredAt.UTC(),
	}

	extras := map[string]any{}
	_ = json.Unmarshal(args.Data, &extras)

	out["name"], out["ref"] = extras["name"], extras["ref"]

	return out
}
