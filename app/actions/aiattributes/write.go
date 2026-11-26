/*
|--------------------------------------------------------------------------
| Writing Conversation Attribute Values
|--------------------------------------------------------------------------
|
| Teammates, the API, workflows, imports and the AI write a
| conversation's values the same way, inside the caller's transaction.
| Every value is checked first, so a refusal sets nothing; then each
| change is kept with who made it, joins the timeline and goes out as
| an event workflows start on.
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

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Writing is values to write to a conversation, by key, and who writes
them: SetByMember, SetByWorkflow, SetByImport or SetByAI.
*/
type Writing struct {
	Conversation models.Conversation
	Values       map[string]any
	By           string
	// Reasons and Sureness are why the AI chose each value and how sure
	// it was, by key, kept beside who set it.
	Reasons  map[string]string
	Sureness map[string]float64
}

/*
stored is what a conversation holds: the attributes that may be
written, its values and who set each.
*/
type stored struct {
	defs    []models.AiConversationAttribute
	values  map[string]any
	sources map[string]Source
}

/*
Write applies values to a conversation in the writer's transaction:
each must fit its active attribute, a nil clears it. A teammate may not
set what only the AI fills, nor what only teams they are not in see,
and the AI never replaces what someone else set. Each change joins the
timeline and dispatches the change events, then the changes come back
in key order.
*/
func Write(ctx context.Context, w *conversations.Writer, in Writing) ([]Change, error) {
	c, by := in.Conversation, in.By
	if len(in.Values) == 0 {
		return nil, nil
	}

	all, defs, err := writable(ctx, w, in)
	if err != nil {
		return nil, err
	}

	row, err := w.Q.LockConversationAIValues(ctx, models.LockConversationAIValuesParams{
		WorkspaceID: c.WorkspaceID, ConversationID: c.ID,
	})
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return nil, fmt.Errorf("lock conversation attribute values: %w", err)
	}

	held := stored{defs: defs, values: decodeValues(row.Values), sources: decodeSources(row.Sources)}

	fitted, err := fitAll(defs, in.Values, by)
	if err != nil {
		return nil, err
	}

	if err := filesExist(ctx, w.Q, c.WorkspaceID, wantedFiles(defs, fitted)); err != nil {
		return nil, err
	}

	next, changes := held.apply(fitted, by)
	if len(changes) == 0 {
		return nil, nil
	}

	for _, ch := range changes {
		held.sources[ch.Key] = sourceOf(w, in, ch.Key)
	}

	params, err := saving(c, next, held.sources)
	if err != nil {
		return nil, err
	}

	params.AttachmentIds, params.Now = fileIDs(all, next), w.Now
	if err := w.Q.SaveConversationAttributeValues(ctx, params); err != nil {
		return nil, fmt.Errorf("save conversation attribute values: %w", err)
	}

	return changes, announce(ctx, w, in, changes)
}

/*
writable returns the workspace's attributes, and those the writer may
write: a teammate only those their teams see.
*/
func writable(
	ctx context.Context, w *conversations.Writer, in Writing,
) ([]models.AiConversationAttribute, []models.AiConversationAttribute, error) {
	ws := in.Conversation.WorkspaceID

	all, err := w.Q.ListAIConversationAttributes(ctx, ws)
	if err != nil {
		return nil, nil, fmt.Errorf("list conversation attributes: %w", err)
	}

	if in.By != SetByMember {
		return all, all, nil
	}

	teams, err := conversations.MemberTeamIDs(ctx, w.Q, ws, w.Actor.ID)
	if err != nil {
		return nil, nil, err
	}

	seen := slices.DeleteFunc(slices.Clone(all), func(d models.AiConversationAttribute) bool {
		return !conversations.SeesAttribute(d.TeamIds, teams)
	})

	return all, seen, nil
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

	_, err := Write(ctx, w, Writing{Conversation: c, Values: values, By: by})

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
func (h stored) apply(fitted map[string]any, by string) (map[string]any, []Change) {
	next := maps.Clone(h.values)

	var changes []Change

	for _, d := range h.defs {
		value, ok := fitted[d.Key]
		if !ok {
			continue
		}

		if s, set := h.sources[d.Key]; by == SetByAI && set && s.By != SetByAI {
			continue
		}

		was, had := h.values[d.Key]
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
func sourceOf(w *conversations.Writer, in Writing, key string) Source {
	s := Source{By: in.By, At: w.Now, Reason: in.Reasons[key]}
	if sure, ok := in.Sureness[key]; ok {
		s.Sureness = &sure
	}

	switch in.By {
	case SetByMember:
		s.ID, s.Name = models.PublicID(models.PrefixMember, w.Actor.ID), w.Actor.Name
	case SetByWorkflow:
		s.Name = w.Actor.Name
	}

	return s
}

/*
saving encodes a conversation's values and who set them for saving.
*/
func saving(
	c models.Conversation, values map[string]any, sources map[string]Source,
) (models.SaveConversationAttributeValuesParams, error) {
	raw, err := json.Marshal(values)
	if err != nil {
		return models.SaveConversationAttributeValuesParams{}, fmt.Errorf("encode attribute values: %w", err)
	}

	by, err := json.Marshal(sources)
	if err != nil {
		return models.SaveConversationAttributeValuesParams{}, fmt.Errorf("encode attribute sources: %w", err)
	}

	return models.SaveConversationAttributeValuesParams{
		ConversationID: c.ID, WorkspaceID: c.WorkspaceID, Values: raw, Sources: by,
	}, nil
}

/*
announce records each change on the timeline and dispatches it: as a
change of a conversation attribute, and for the AI's own changes also
as the AI attribute change workflows started on before.
*/
func announce(ctx context.Context, w *conversations.Writer, in Writing, changes []Change) error {
	c, by := in.Conversation, in.By
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
