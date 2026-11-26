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
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/messages"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
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
	ProblemUnknown     = "unknown_attribute"
	ProblemArchived    = "archived_attribute"
	ProblemUnknownFile = "unknown_file"
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
	Files     map[string]messages.File
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

		out, err = a.foundFor(ctx, q, m, c.ID)

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

		if _, err := Write(ctx, w, Writing{Conversation: c, Values: in.Values, By: SetByMember}); err != nil {
			return err
		}

		out, err = a.foundFor(ctx, q, m, c.ID)

		return err
	})

	return out, err
}

/*
foundFor reads a conversation's attributes the member's teams see.
*/
func (a *Attributes) foundFor(
	ctx context.Context, q *models.Queries, m authz.Membership, conversationID ids.UUID,
) (Found, error) {
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

	err = a.present(ctx, q, m.Workspace.ID, &out)

	return out, err
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
