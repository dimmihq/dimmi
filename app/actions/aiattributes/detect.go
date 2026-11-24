/*
|--------------------------------------------------------------------------
| Detecting AI Attributes
|--------------------------------------------------------------------------
|
| Each customer message asks for the conversation to be read again, a
| short while later so a burst of messages is read once, and never more
| than once a minute. The decision model reads the whole conversation
| and fills in the active attributes. A value it is sure of replaces the
| one kept; one it is unsure of leaves the kept value as it was, so a
| conversation never loses what it was found to be about because the
| customer said thanks. Every value set or changed dispatches an event
| workflows start on. The model is asked with no transaction open.
|
*/

package aiattributes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/ai/aidecision"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
How often a conversation is read: a while after a customer message, so
a burst is read once, and at most once a minute.
*/
const (
	debounce   = 20 * time.Second
	leastGap   = time.Minute
	maxReadMsg = 100
)

/*
keyPattern is what an attribute key may look like.
*/
var keyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

/*
Listen asks for a conversation to be read again after each of the
customer's messages.
*/
func (a *Attributes) Listen(d *events.Dispatcher) {
	events.ListenQueued(d, func(ev appevents.MessageCreated) queue.Args {
		if ev.AuthorType != string(enums.AuthorTypeContact) || ev.Type != string(enums.MessageTypeMessage) ||
			!ev.Substantive {
			return nil
		}

		return jobs.DetectConversationAttributesArgs{
			WorkspaceID: ev.WorkspaceID, ConversationID: ev.ConversationID, NotBefore: a.Clock.Now().Add(debounce),
		}
	})
}

/*
reading is what one check reads: the attributes to fill, the values
kept, the conversation and the newest customer message.
*/
type reading struct {
	defs    []models.AiConversationAttribute
	kept    map[string]any
	text    string
	newest  ids.UUID
	checked *time.Time
}

/*
Detect reads the conversation and keeps the values the decision model
is sure of.
*/
func (a *Attributes) Detect(ctx context.Context, args jobs.DetectConversationAttributesArgs) error {
	if a.Decisions == nil {
		return nil
	}

	ctx = tenancy.With(ctx, args.WorkspaceID.String())

	r, err := a.read(ctx, args)
	if err != nil || r == nil {
		return err
	}

	if r.checked != nil {
		if wait := r.checked.Add(leastGap).Sub(a.Clock.Now()); wait > 0 {
			return queue.Postpone(wait)
		}
	}

	conv := args.ConversationID
	got, err := a.Decisions.Extract(ctx, aidecision.Extraction{
		Subject: aidecision.Subject{WorkspaceID: args.WorkspaceID, ConversationID: &conv, Context: r.text},
		Fields:  fieldsOf(r.defs),
	})
	if errors.Is(err, aidecision.ErrUnavailable) {
		return nil
	}

	if err != nil {
		return err
	}

	if err := a.keep(ctx, args, r, got.Values); err != nil {
		return err
	}

	return a.again(ctx, args, r.newest)
}

/*
again reads the conversation once more, a minute on, when the customer
wrote while it was being read: their message's own check found this
one running and was dropped.
*/
func (a *Attributes) again(ctx context.Context, args jobs.DetectConversationAttributesArgs, read ids.UUID) error {
	var newer bool

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		m, err := models.New(tx).LatestContactMessage(ctx, models.LatestContactMessageParams{
			WorkspaceID: args.WorkspaceID, ConversationID: args.ConversationID,
		})
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}

		newer = err == nil && m.ID != read

		return err
	})
	if err != nil || !newer {
		return err
	}

	return queue.Postpone(leastGap)
}

/*
read reads what a check needs, nil when there is nothing to do: no
active attribute, no customer message, or none since the last check.
*/
func (a *Attributes) read(ctx context.Context, args jobs.DetectConversationAttributesArgs) (*reading, error) {
	var r *reading

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		defs, err := q.ListActiveAIConversationAttributes(ctx, args.WorkspaceID)
		if err != nil || len(defs) == 0 {
			return err
		}

		c, msgs, err := a.conversation(ctx, q, args)
		if err != nil || c == nil {
			return err
		}

		newest, ok := newestCustomer(msgs)
		if !ok {
			return nil
		}

		row, err := q.FindConversationAIValues(ctx, models.FindConversationAIValuesParams{
			WorkspaceID: args.WorkspaceID, ConversationID: args.ConversationID,
		})
		if err != nil && !errors.Is(err, database.ErrNotFound) {
			return fmt.Errorf("find ai values: %w", err)
		}

		if row.CheckedMessageID != nil && *row.CheckedMessageID == newest {
			return nil
		}

		slices.Reverse(msgs)
		r = &reading{
			defs: defs, kept: decodeValues(row.Values), text: aidecision.Transcript(c.Subject, msgs),
			newest: newest, checked: row.CheckedAt,
		}

		return nil
	})

	return r, err
}

/*
conversation reads the conversation and its newest messages, newest
first; nil for one gone, deleted or spam.
*/
func (a *Attributes) conversation(
	ctx context.Context, q *models.Queries, args jobs.DetectConversationAttributesArgs,
) (*models.Conversation, []models.Message, error) {
	c, err := q.FindConversation(ctx, models.FindConversationParams{
		WorkspaceID: args.WorkspaceID, ID: args.ConversationID,
	})
	if errors.Is(err, database.ErrNotFound) || err == nil && (c.Spam || c.DeletedAt != nil) {
		return nil, nil, nil
	}

	if err != nil {
		return nil, nil, fmt.Errorf("find conversation: %w", err)
	}

	now := a.Clock.Now()

	msgs, err := q.ListLatestPublicMessages(ctx, models.ListLatestPublicMessagesParams{
		WorkspaceID: args.WorkspaceID, ConversationID: args.ConversationID, Now: &now, RowLimit: maxReadMsg,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("list messages: %w", err)
	}

	return &c, msgs, nil
}

/*
keep merges the values found into the kept ones, records the message
they were read at and tells workflows what changed.
*/
func (a *Attributes) keep(
	ctx context.Context, args jobs.DetectConversationAttributesArgs, r *reading, found map[string]any,
) error {
	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, now := models.New(tx), a.Clock.Now()

		row, err := q.LockConversationAIValues(ctx, models.LockConversationAIValuesParams{
			WorkspaceID: args.WorkspaceID, ConversationID: args.ConversationID,
		})
		if err != nil && !errors.Is(err, database.ErrNotFound) {
			return fmt.Errorf("lock ai values: %w", err)
		}

		next, changes := merge(r.defs, decodeValues(row.Values), found)

		raw, err := json.Marshal(next)
		if err != nil {
			return fmt.Errorf("encode ai values: %w", err)
		}

		if err := q.SaveConversationAIValues(ctx, models.SaveConversationAIValuesParams{
			ConversationID: args.ConversationID, WorkspaceID: args.WorkspaceID, Values: raw,
			CheckedMessageID: &r.newest, Now: &now,
		}); err != nil {
			return fmt.Errorf("save ai values: %w", err)
		}

		for _, ch := range changes {
			ev := appevents.ConversationAIAttributeChanged{
				Base: appevents.Base{
					ID: ids.New(), WorkspaceID: args.WorkspaceID, OccurredAt: now,
					Actor: appevents.Actor{Type: "system"},
				},
				ConversationRef: appevents.ConversationRef{ConversationID: args.ConversationID},
				Key:             ch.key, From: ch.from, To: ch.to,
			}
			if err := a.Events.Dispatch(ctx, tx, ev); err != nil {
				return err
			}
		}

		return nil
	})
}

/*
change is one value set or changed.
*/
type change struct {
	key      string
	from, to any
}

/*
merge keeps every value found that fits its attribute, over the value
kept, and lists what changed.
*/
func merge(defs []models.AiConversationAttribute, kept, found map[string]any) (map[string]any, []change) {
	next := maps.Clone(kept)

	var changes []change

	for _, d := range defs {
		value, ok := found[d.Key]
		if !ok || !aidecision.Fits(fieldOf(d), value) || same(kept[d.Key], value) {
			continue
		}

		next[d.Key] = value
		changes = append(changes, change{key: d.Key, from: kept[d.Key], to: value})
	}

	return next, changes
}

/*
newestCustomer returns the newest message the customer wrote, from
messages newest first.
*/
func newestCustomer(msgs []models.Message) (ids.UUID, bool) {
	for _, m := range msgs {
		if m.AuthorType == string(enums.AuthorTypeContact) {
			return m.ID, true
		}
	}

	return ids.UUID{}, false
}

/*
fieldsOf turns attributes into the fields the decision step fills.
*/
func fieldsOf(defs []models.AiConversationAttribute) []aidecision.Field {
	out := make([]aidecision.Field, 0, len(defs))
	for _, d := range defs {
		out = append(out, fieldOf(d))
	}

	return out
}

/*
fieldOf turns one attribute into a field.
*/
func fieldOf(d models.AiConversationAttribute) aidecision.Field {
	return aidecision.Field{
		Key: d.Key, Label: d.Label, Description: d.Description, Type: d.DataType, Options: d.Options,
	}
}

/*
decodeValues reads kept values, none for nothing kept.
*/
func decodeValues(raw json.RawMessage) map[string]any {
	out := map[string]any{}
	_ = json.Unmarshal(raw, &out)

	return out
}

/*
same reports whether two values are equal as JSON.
*/
func same(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)

	return bytes.Equal(x, y)
}
