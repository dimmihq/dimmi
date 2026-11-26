/*
|--------------------------------------------------------------------------
| Conversation Attribute Values
|--------------------------------------------------------------------------
|
| A conversation's values, read beside the attributes that say what
| each means and who set it. Teammates, the API, workflows, imports and
| the AI all write them the same way: each value must fit its
| attribute, a null clears it, and every change joins the timeline and
| goes out as an event workflows start on. The AI never replaces a value
| someone else set. Values of archived attributes are kept and still
| read, so a workflow or view that named one keeps working on what it
| last held; they are no longer written.
|
*/

package aiattributes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sort"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Who set a value.
*/
const (
	SetByAI       = "ai"
	SetByMember   = "member"
	SetByWorkflow = "workflow"
	SetByImport   = "import"
)

/*
The problems a value may have besides those of its type.
*/
const (
	ProblemUnknown  = "unknown_attribute"
	ProblemArchived = "archived_attribute"
)

/*
ErrValueInvalid refuses values that do not fit their attributes, with
the problem of each by key.
*/
var ErrValueInvalid = fault.New(fault.Invalid, "attribute_values_invalid",
	"Some values do not fit their conversation attributes.").OnField("values")

/*
ErrFilledByAI refuses a teammate's value for an attribute only the AI
fills.
*/
var ErrFilledByAI = fault.New(fault.Invalid, "attribute_filled_by_ai",
	"Only the AI fills this conversation attribute.").OnField("values")

/*
Source is who set a value and when: the AI, a teammate (with their id
and name), a workflow (with its name) or an import.
*/
type Source struct {
	By   string    `json:"by"`
	ID   string    `json:"id,omitempty"`
	Name string    `json:"name,omitempty"`
	At   time.Time `json:"at"`
}

/*
Value is one attribute of a conversation with its value, nil when none
is set, and who set it.
*/
type Value struct {
	Attribute models.AiConversationAttribute
	Value     any
	Source    *Source
}

/*
Found is a conversation's attributes and when the AI last read it.
*/
type Found struct {
	Values    []Value
	CheckedAt *time.Time
}

/*
SetInput is a teammate's or an API caller's values for a conversation,
by key; a nil value clears it.
*/
type SetInput struct {
	Membership authz.Membership
	Ref        conversations.Ref
	Values     map[string]any
	MutationID string
}

/*
Change is one value set, changed or cleared.
*/
type Change struct {
	Key      string
	Label    string
	From, To any
}

/*
Values returns the attributes of a conversation the member may see.
*/
func (a *Attributes) Values(ctx context.Context, m authz.Membership, ref conversations.Ref) (Found, error) {
	if err := conversations.CanRead(m); err != nil {
		return Found{}, err
	}

	var out Found

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		c, err := conversations.Find(ctx, q, m, ref)
		if err != nil {
			return err
		}

		out, err = foundFor(ctx, q, m, c.ID)

		return err
	})

	return out, err
}

/*
Set writes a teammate's or an API caller's values and returns the
conversation's attributes as they now are.
*/
func (a *Attributes) Set(ctx context.Context, in SetInput) (Found, error) {
	m := in.Membership
	if err := conversations.CanWrite(m); err != nil {
		return Found{}, err
	}

	var out Found

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		c, err := conversations.Find(ctx, q, m, in.Ref)
		if err != nil {
			return err
		}

		w, err := conversations.NewWriter(ctx, tx, conversations.Services{Events: a.Events, Clock: a.Clock}, m)
		if err != nil {
			return err
		}

		w.MutationID = in.MutationID

		if _, err := Write(ctx, w, c, in.Values, SetByMember); err != nil {
			return err
		}

		out, err = foundFor(ctx, q, m, c.ID)

		return err
	})

	return out, err
}

/*
foundFor reads a conversation's attributes the member's teams see.
*/
func foundFor(ctx context.Context, q *models.Queries, m authz.Membership, conversationID ids.UUID) (Found, error) {
	out, err := found(ctx, q, m.Workspace.ID, conversationID)
	if err != nil {
		return out, err
	}

	teams, err := conversations.MemberTeamIDs(ctx, q, m.Workspace.ID, m.Member.ID)
	if err != nil {
		return out, err
	}

	out.Values = slices.DeleteFunc(out.Values, func(v Value) bool {
		return !conversations.SeesAttribute(v.Attribute.TeamIds, teams)
	})

	return out, nil
}

/*
found reads a conversation's attributes: every active one, and each
archived one that still holds a value.
*/
func found(ctx context.Context, q *models.Queries, workspaceID, conversationID ids.UUID) (Found, error) {
	defs, err := q.ListAIConversationAttributes(ctx, workspaceID)
	if err != nil {
		return Found{}, fmt.Errorf("list conversation attributes: %w", err)
	}

	row, err := q.FindConversationAIValues(ctx, models.FindConversationAIValuesParams{
		WorkspaceID: workspaceID, ConversationID: conversationID,
	})
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return Found{}, fmt.Errorf("find conversation attribute values: %w", err)
	}

	kept, sources := decodeValues(row.Values), decodeSources(row.Sources)
	out := Found{CheckedAt: row.CheckedAt}

	for _, d := range defs {
		v, ok := kept[d.Key]
		if !ok && d.Status != "active" {
			continue
		}

		value := Value{Attribute: d, Value: v}
		if s, ok := sources[d.Key]; ok {
			value.Source = &s
		}

		out.Values = append(out.Values, value)
	}

	return out, nil
}

/*
Write applies values to a conversation in the writer's transaction, as
the one named by: each must fit its active attribute, a nil clears it.
A teammate may not set what only the AI fills, nor what only teams they
are not in see, and the AI never
replaces what someone else set. Each change joins the timeline and
dispatches the change events, then the changes come back in key order.
*/
func Write(
	ctx context.Context, w *conversations.Writer, c models.Conversation, values map[string]any, by string,
) ([]Change, error) {
	if len(values) == 0 {
		return nil, nil
	}

	defs, err := w.Q.ListAIConversationAttributes(ctx, c.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("list conversation attributes: %w", err)
	}

	if by == SetByMember {
		teams, err := conversations.MemberTeamIDs(ctx, w.Q, c.WorkspaceID, w.Actor.ID)
		if err != nil {
			return nil, err
		}

		defs = slices.DeleteFunc(defs, func(d models.AiConversationAttribute) bool {
			return !conversations.SeesAttribute(d.TeamIds, teams)
		})
	}

	row, err := w.Q.LockConversationAIValues(ctx, models.LockConversationAIValuesParams{
		WorkspaceID: c.WorkspaceID, ConversationID: c.ID,
	})
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return nil, fmt.Errorf("lock conversation attribute values: %w", err)
	}

	kept, sources := decodeValues(row.Values), decodeSources(row.Sources)

	fitted, err := fitAll(defs, values, by)
	if err != nil {
		return nil, err
	}

	next, changes := apply(defs, kept, sources, fitted, by)
	if len(changes) == 0 {
		return nil, nil
	}

	source := sourceOf(w, by)
	for _, ch := range changes {
		sources[ch.Key] = source
	}

	if err := save(ctx, w.Q, c, next, sources, w.Now); err != nil {
		return nil, err
	}

	return changes, announce(ctx, w, c, changes, by)
}

/*
WriteNew sets a new conversation's values: by the import when the
writer replays imported history, else by the writer's teammate.
*/
func WriteNew(ctx context.Context, w *conversations.Writer, c models.Conversation, values map[string]any) error {
	by := SetByMember
	if w.Imported {
		by = SetByImport
	}

	_, err := Write(ctx, w, c, values, by)

	return err
}

/*
fitAll checks the values against their attributes and returns them in
their stored shape, nil for a value cleared.
*/
func fitAll(defs []models.AiConversationAttribute, values map[string]any, by string) (map[string]any, error) {
	out, problems := map[string]any{}, map[string]string{}

	for key, value := range values {
		i := slices.IndexFunc(defs, func(d models.AiConversationAttribute) bool { return d.Key == key })

		switch {
		case i < 0:
			problems[key] = ProblemUnknown
		case defs[i].Status != "active":
			problems[key] = ProblemArchived
		case by == SetByMember && defs[i].FilledBy == FilledByAI:
			return nil, ErrFilledByAI.With("key", key)
		case value == nil:
			out[key] = nil
		default:
			v, problem := fit(defs[i], value)
			if problem != "" {
				problems[key] = problem
				continue
			}

			out[key] = v
		}
	}

	if len(problems) > 0 {
		return nil, ErrValueInvalid.With("problems", problems)
	}

	return out, nil
}

/*
apply merges the fitted values into those kept and lists what changed,
leaving alone what the AI may not replace.
*/
func apply(
	defs []models.AiConversationAttribute, kept map[string]any, sources map[string]Source, fitted map[string]any,
	by string,
) (map[string]any, []Change) {
	next := maps.Clone(kept)

	var changes []Change

	for _, d := range defs {
		value, ok := fitted[d.Key]
		if !ok {
			continue
		}

		if s, set := sources[d.Key]; by == SetByAI && set && s.By != SetByAI {
			continue
		}

		was, had := kept[d.Key]
		if value == nil && !had || had && same(was, value) {
			continue
		}

		if value == nil {
			delete(next, d.Key)
		} else {
			next[d.Key] = value
		}

		changes = append(changes, Change{Key: d.Key, Label: d.Label, From: was, To: value})
	}

	sort.Slice(changes, func(i, j int) bool { return changes[i].Key < changes[j].Key })

	return next, changes
}

/*
sourceOf names who the writer sets values as.
*/
func sourceOf(w *conversations.Writer, by string) Source {
	s := Source{By: by, At: w.Now}
	switch by {
	case SetByMember:
		s.ID, s.Name = models.PublicID(models.PrefixMember, w.Actor.ID), w.Actor.Name
	case SetByWorkflow:
		s.Name = w.Actor.Name
	}

	return s
}

/*
save keeps a conversation's values and who set them.
*/
func save(
	ctx context.Context, q *models.Queries, c models.Conversation, values map[string]any, sources map[string]Source,
	now time.Time,
) error {
	raw, err := json.Marshal(values)
	if err != nil {
		return fmt.Errorf("encode conversation attribute values: %w", err)
	}

	by, err := json.Marshal(sources)
	if err != nil {
		return fmt.Errorf("encode conversation attribute sources: %w", err)
	}

	if err := q.SaveConversationAttributeValues(ctx, models.SaveConversationAttributeValuesParams{
		ConversationID: c.ID, WorkspaceID: c.WorkspaceID, Values: raw, Sources: by, Now: now,
	}); err != nil {
		return fmt.Errorf("save conversation attribute values: %w", err)
	}

	return nil
}

/*
announce records each change on the timeline and dispatches it: as a
change of a conversation attribute, and for the AI's own changes also
as the AI attribute change workflows started on before.
*/
func announce(ctx context.Context, w *conversations.Writer, c models.Conversation, changes []Change, by string) error {
	for _, ch := range changes {
		if by != SetByImport {
			if err := w.Record(ctx, c, "conversation.attribute_changed", map[string]any{
				"attribute": map[string]any{"key": ch.Key, "label": ch.Label}, "from": ch.From, "to": ch.To,
				"by": by,
			}); err != nil {
				return err
			}
		}

		base := w.Base(c.WorkspaceID)
		ref := appevents.ConversationRef{ConversationID: c.ID}

		if err := w.Dispatch(ctx, appevents.ConversationAttributeChanged{
			Base: base, ConversationRef: ref, Key: ch.Key, From: ch.From, To: ch.To, By: by,
		}); err != nil {
			return err
		}

		if by != SetByAI {
			continue
		}

		base.ID = ids.New()
		if err := w.Dispatch(ctx, appevents.ConversationAIAttributeChanged{
			Base: base, ConversationRef: ref, Key: ch.Key, From: ch.From, To: ch.To,
		}); err != nil {
			return err
		}
	}

	return nil
}

/*
Kept returns the values of a conversation, by key, for a caller inside
the workspace's transaction.
*/
func Kept(ctx context.Context, q *models.Queries, workspaceID, conversationID ids.UUID) (map[string]any, error) {
	row, err := q.FindConversationAIValues(ctx, models.FindConversationAIValuesParams{
		WorkspaceID: workspaceID, ConversationID: conversationID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return map[string]any{}, nil
	}

	if err != nil {
		return nil, fmt.Errorf("find conversation attribute values: %w", err)
	}

	return decodeValues(row.Values), nil
}

/*
decodeSources reads who set each value, none for nothing kept.
*/
func decodeSources(raw json.RawMessage) map[string]Source {
	out := map[string]Source{}
	_ = json.Unmarshal(raw, &out)

	return out
}

/*
aiWriter is the writer the AI's values are written with.
*/
func aiWriter(q *models.Queries, tx *database.Tx, a *Attributes) *conversations.Writer {
	return &conversations.Writer{
		Q: q, Tx: tx, Events: a.Events, Now: a.Clock.Now(),
		Actor: conversations.Actor{Type: string(enums.AuthorTypeSystem), Name: "AI"},
	}
}
