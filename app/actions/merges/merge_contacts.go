/*
|--------------------------------------------------------------------------
| Merge Contacts
|--------------------------------------------------------------------------
|
| The same person often shows up twice: a lead who typed their address in
| the widget and the user the host app later identified. Merging keeps
| the target and folds the source into it. Conversations, identities,
| tags and event counts move over; the target's details win and the
| source only fills what the target lacks. The source stays as a
| tombstone pointing at the target, so old links still resolve, and the
| audit log keeps who merged whom. Two users the host app knows by
| different ids are never merged, since each would keep signing in as
| themselves.
|
*/

package merges

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
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
MergeContactsInput is who merges which contact into which.
*/
type MergeContactsInput struct {
	Membership authz.Membership
	TargetID   ids.UUID
	SourceID   ids.UUID
}

/*
MergeContacts merges a contact into another.

Rules:
  - The caller needs contacts:write.
  - Both contacts are live and not already merged, and differ.
  - Two users with different external ids are refused.
  - The target's details win; the source fills blanks. Custom
    attributes merge key by key the same way.
  - Conversations, places in conversations, identities, tags and event
    summaries move to the target; event counts add up.
  - The source points at the target and keeps no external id.

Events: [appevents.ContactMerged].

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrMergeIntoItself], [exceptions.ErrMergeTwoUsers], or a
wrapped database error.
*/
type MergeContacts struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle merges the source into the target and returns the target as it
now is.
*/
func (a *MergeContacts) Handle(ctx context.Context, in MergeContactsInput) (models.Contact, error) {
	if !in.Membership.Can("contacts:write") {
		return models.Contact{}, exceptions.ErrForbidden
	}

	if in.TargetID == in.SourceID {
		return models.Contact{}, exceptions.ErrMergeIntoItself
	}

	var out models.Contact

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		out, err = a.merge(ctx, tx, in)

		return err
	})

	return out, err
}

/*
move is one merge of contacts: the workspace, the survivor, the contact
merged away and when.
*/
type move struct {
	ws     ids.UUID
	target ids.UUID
	source ids.UUID
	now    time.Time
}

/*
merge folds the source into the target within the transaction and
returns the target as it now is.
*/
func (a *MergeContacts) merge(ctx context.Context, tx *database.Tx, in MergeContactsInput) (models.Contact, error) {
	q := models.New(tx)
	mv := move{ws: in.Membership.Workspace.ID, target: in.TargetID, source: in.SourceID, now: a.Clock.Now()}

	target, source, err := lockPair(ctx, q, mv)
	if err != nil {
		return target, err
	}

	if target.ExternalID != nil && source.ExternalID != nil && *target.ExternalID != *source.ExternalID {
		return target, exceptions.ErrMergeTwoUsers
	}

	err = q.RetireMergedContact(ctx, models.RetireMergedContactParams{
		WorkspaceID: mv.ws, SourceID: mv.source, TargetID: mv.target, Now: mv.now,
	})
	if err != nil {
		return target, fmt.Errorf("retire merged contact: %w", err)
	}

	params, err := mergedDetails(target, source)
	if err != nil {
		return target, err
	}

	params.Now = mv.now

	out, err := q.SaveMergedContact(ctx, params)
	if err != nil {
		return out, fmt.Errorf("save merged contact: %w", err)
	}

	moved, err := moveBelongings(ctx, q, mv)
	if err != nil {
		return out, err
	}

	return out, a.Events.Dispatch(ctx, tx, appevents.ContactMerged{
		Base: appevents.Base{
			ID: ids.New(), WorkspaceID: mv.ws, OccurredAt: mv.now,
			Actor: appevents.Actor{Type: "member", ID: in.Membership.Member.ID},
		},
		ContactID: mv.target, SourceID: mv.source, Conversations: moved.conversations, Identities: moved.identities,
	})
}

/*
lockPair locks both contacts in id order and refuses one that is gone,
deleted or already merged.
*/
func lockPair(ctx context.Context, q *models.Queries, mv move) (models.Contact, models.Contact, error) {
	rows, err := q.LockContactsForMerge(ctx, models.LockContactsForMergeParams{
		WorkspaceID: mv.ws, Ids: []ids.UUID{mv.target, mv.source},
	})
	if err != nil {
		return models.Contact{}, models.Contact{}, fmt.Errorf("lock contacts: %w", err)
	}

	var target, source *models.Contact

	for i := range rows {
		row := &rows[i]
		if row.DeletedAt != nil || row.MergedIntoID != nil {
			continue
		}

		switch row.ID {
		case mv.target:
			target = row
		case mv.source:
			source = row
		}
	}

	if target == nil || source == nil {
		return models.Contact{}, models.Contact{}, exceptions.ErrNotFound
	}

	return *target, *source, nil
}

/*
mergedDetails works out the target's details after the merge: its own
where it has them, the source's where it has none.
*/
func mergedDetails(t, s models.Contact) (models.SaveMergedContactParams, error) {
	attributes, err := mergedAttributes(t.CustomAttributes, s.CustomAttributes)
	if err != nil {
		return models.SaveMergedContactParams{}, err
	}

	p := models.SaveMergedContactParams{
		WorkspaceID: t.WorkspaceID, ID: t.ID, Kind: t.Kind,
		ExternalID: fill(t.ExternalID, s.ExternalID), Email: fill(t.Email, s.Email), EmailVerified: t.EmailVerified,
		Name: fill(t.Name, s.Name), Phone: fill(t.Phone, s.Phone), AvatarUrl: fill(t.AvatarUrl, s.AvatarUrl),
		CompanyID: fill(t.CompanyID, s.CompanyID), CustomAttributes: attributes,
		TrustedAttributeKeys: trustedKeys(t, s), Locale: fill(t.Locale, s.Locale),
		Timezone: fill(t.Timezone, s.Timezone), Location: t.Location, Notes: fill(t.Notes, s.Notes),
		FirstSeenAt: earliest(t.FirstSeenAt, s.FirstSeenAt), LastSeenAt: latest(t.LastSeenAt, s.LastSeenAt),
		Placeholder: t.Placeholder && s.Placeholder,
	}

	if t.Email == nil && s.Email != nil {
		p.EmailVerified = s.EmailVerified
	}

	if p.ExternalID != nil {
		p.Kind = "user"
	}

	if isEmptyJSON(t.Location) {
		p.Location = s.Location
	}

	return p, nil
}

/*
fill returns the target's value, or the source's when the target has
none.
*/
func fill[T any](target, source *T) *T {
	if target != nil {
		return target
	}

	return source
}

/*
earliest returns the earlier of two times, either of which may be
missing.
*/
func earliest(a, b *time.Time) *time.Time {
	if a == nil || (b != nil && b.Before(*a)) {
		return b
	}

	return a
}

/*
latest returns the later of two times, either of which may be missing.
*/
func latest(a, b *time.Time) *time.Time {
	if a == nil || (b != nil && b.After(*a)) {
		return b
	}

	return a
}

/*
isEmptyJSON reports whether a JSON value holds nothing.
*/
func isEmptyJSON(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)

	return len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) || bytes.Equal(trimmed, []byte("{}"))
}

/*
mergedAttributes merges the custom attributes key by key, the target's
value winning.
*/
func mergedAttributes(target, source json.RawMessage) (json.RawMessage, error) {
	merged := map[string]json.RawMessage{}

	for _, raw := range []json.RawMessage{source, target} {
		if isEmptyJSON(raw) {
			continue
		}

		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return nil, fmt.Errorf("read custom attributes: %w", err)
		}

		maps.Copy(merged, fields)
	}

	out, err := json.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("encode custom attributes: %w", err)
	}

	return out, nil
}

/*
trustedKeys keeps the target's trusted keys, and the source's for the
attributes whose value came from the source.
*/
func trustedKeys(t, s models.Contact) []string {
	var own map[string]json.RawMessage
	if !isEmptyJSON(t.CustomAttributes) {
		_ = json.Unmarshal(t.CustomAttributes, &own)
	}

	keys := append([]string{}, t.TrustedAttributeKeys...)

	for _, key := range s.TrustedAttributeKeys {
		if _, held := own[key]; !held && !slices.Contains(keys, key) {
			keys = append(keys, key)
		}
	}

	return keys
}

/*
belongings counts what moved from the source to the target.
*/
type belongings struct {
	conversations int64
	identities    int64
}

/*
moveBelongings moves the source's conversations, places in other
conversations, identities, tags and event summaries to the target.
*/
func moveBelongings(ctx context.Context, q *models.Queries, mv move) (belongings, error) {
	var out belongings

	for _, step := range belongingSteps(ctx, q, mv, &out) {
		if err := step(); err != nil {
			return out, fmt.Errorf("move contact belongings: %w", err)
		}
	}

	return out, nil
}

/*
belongingSteps returns the moves of the source's belongings, in order,
each counting what it moved into out.
*/
func belongingSteps(ctx context.Context, q *models.Queries, mv move, out *belongings) []func() error {
	var err error

	ws, source, target, now := mv.ws, mv.source, mv.target, mv.now

	return []func() error{
		func() error {
			out.conversations, err = q.MoveContactConversations(ctx, models.MoveContactConversationsParams{
				WorkspaceID: ws, SourceID: source, TargetID: target, Now: now,
			})

			return err
		},
		func() error {
			return q.MoveContactParticipations(ctx, models.MoveContactParticipationsParams{
				WorkspaceID: ws, SourceID: source, TargetID: target,
			})
		},
		func() error {
			return q.DropContactParticipations(ctx, models.DropContactParticipationsParams{
				WorkspaceID: ws, SourceID: source,
			})
		},
		func() error {
			out.identities, err = q.MoveContactIdentities(ctx, models.MoveContactIdentitiesParams{
				WorkspaceID: ws, SourceID: source, TargetID: target, Now: now,
			})

			return err
		},
		func() error {
			return q.MoveContactTags(ctx, models.MoveContactTagsParams{
				WorkspaceID: ws, SourceID: source, TargetID: target,
			})
		},
		func() error {
			return q.FoldContactEventSummaries(ctx, models.FoldContactEventSummariesParams{
				WorkspaceID: ws, SourceID: source, TargetID: target, Now: now,
			})
		},
	}
}
