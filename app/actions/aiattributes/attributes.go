/*
|--------------------------------------------------------------------------
| AI Conversation Attributes
|--------------------------------------------------------------------------
|
| A workspace defines once the things it wants known about every
| conversation: a name, a type (text, number, date, yes or no, or one of
| a few choices) and what it means. The decision model fills them in
| and checks them again as the customer writes. Workflows branch and
| start on them and inbox views filter by them. A key and a type never
| change once made; an attribute is archived instead of deleted, so
| what read it keeps reading its last values. Reading takes
| conversations:read; changing takes ai_agent:manage.
|
*/

package aiattributes

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/ai/aidecision"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The failures of AI conversation attributes.
*/
var (
	ErrNotFound = fault.New(fault.NotFound, "ai_attribute_not_found", "No AI attribute of this workspace has this id.")
	ErrExists   = fault.New(fault.Conflict, "ai_attribute_exists", "An AI attribute already has this key.").
			OnField("key")
	ErrKey = fault.New(fault.Invalid, "key_invalid",
		"Keys start with a letter and use lowercase letters, digits and underscores, 64 at most.").OnField("key")
	ErrType = fault.New(fault.Invalid, "data_type_invalid", "Pick text, number, date, boolean or choice.").
		OnField("data_type")
	ErrOptions = fault.New(fault.Invalid, "options_invalid",
		"A choice needs 1 to 30 different values of 80 characters at most; other types take none.").OnField("options")
	ErrLabel       = fault.New(fault.Invalid, "label_invalid", "Names are 1 to 80 characters.").OnField("label")
	ErrDescription = fault.New(fault.Invalid, "description_invalid", "Descriptions are 500 characters at most.").
			OnField("description")
	ErrStatus = fault.New(fault.Invalid, "status_invalid", "Status is active or archived.").OnField("status")
	ErrLimit  = fault.New(fault.Invalid, "ai_attribute_limit_reached", "A workspace may have 30 active AI attributes.")
)

/*
Limits of AI attributes.
*/
const (
	maxActive      = 30
	maxLabel       = 80
	maxDescription = 500
	maxChoices     = 30
)

/*
Attributes manages a workspace's AI conversation attributes and keeps
their values. Decisions is the AI decision step that fills them in;
without one nothing is detected.
*/
type Attributes struct {
	DB        *database.DB
	Clock     clock.Clock
	IDs       *ids.Registry
	Events    *events.Dispatcher
	Queue     *queue.Queue
	Decisions aidecision.Service
}

/*
CreateInput defines a new attribute.
*/
type CreateInput struct {
	Membership  authz.Membership
	Key         string
	Label       string
	Description string
	DataType    string
	Options     []string
}

/*
UpdateInput changes an attribute's words, choices and status; nil
leaves each as it is.
*/
type UpdateInput struct {
	Membership  authz.Membership
	ID          ids.UUID
	Label       *string
	Description *string
	Options     []string
	Status      *string
}

/*
List returns the workspace's attributes, active first.
*/
func (a *Attributes) List(ctx context.Context, m authz.Membership) ([]models.AiConversationAttribute, error) {
	if !m.Can("conversations:read") {
		return nil, exceptions.ErrForbidden
	}

	var out []models.AiConversationAttribute

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = models.New(tx).ListAIConversationAttributes(ctx, m.Workspace.ID)

		return err
	})
	if err != nil {
		return nil, fmt.Errorf("list ai attributes: %w", err)
	}

	return out, nil
}

/*
Create defines a new attribute.
*/
func (a *Attributes) Create(ctx context.Context, in CreateInput) (models.AiConversationAttribute, error) {
	m := in.Membership
	if !m.Can("ai_agent:manage") {
		return models.AiConversationAttribute{}, exceptions.ErrForbidden
	}

	if err := checkNew(in); err != nil {
		return models.AiConversationAttribute{}, err
	}

	var out models.AiConversationAttribute

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		if err := roomFor(ctx, q, m.Workspace.ID); err != nil {
			return err
		}

		var err error
		out, err = q.CreateAIConversationAttribute(ctx, models.CreateAIConversationAttributeParams{
			WorkspaceID: m.Workspace.ID, Key: in.Key, Label: strings.TrimSpace(in.Label),
			Description: strings.TrimSpace(in.Description), DataType: in.DataType, Options: trimmed(in.Options),
			Now: a.Clock.Now(),
		})
		if errors.Is(err, database.ErrNotFound) {
			return ErrExists
		}

		return err
	})

	return out, err
}

/*
Update changes an attribute.
*/
func (a *Attributes) Update(ctx context.Context, in UpdateInput) (models.AiConversationAttribute, error) {
	m := in.Membership
	if !m.Can("ai_agent:manage") {
		return models.AiConversationAttribute{}, exceptions.ErrForbidden
	}

	var out models.AiConversationAttribute

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		cur, err := q.FindAIConversationAttribute(ctx, models.FindAIConversationAttributeParams{
			WorkspaceID: m.Workspace.ID, ID: in.ID,
		})
		if errors.Is(err, database.ErrNotFound) {
			return ErrNotFound
		}

		if err != nil {
			return fmt.Errorf("find ai attribute: %w", err)
		}

		next, err := changed(cur, in)
		if err != nil {
			return err
		}

		if cur.Status == "archived" && next.Status == "active" {
			if err := roomFor(ctx, q, m.Workspace.ID); err != nil {
				return err
			}
		}

		out, err = q.UpdateAIConversationAttribute(ctx, models.UpdateAIConversationAttributeParams{
			Label: next.Label, Description: next.Description, Options: next.Options, Status: next.Status,
			Now: a.Clock.Now(), WorkspaceID: m.Workspace.ID, ID: in.ID,
		})

		return err
	})

	return out, err
}

/*
roomFor refuses one more active attribute past the limit.
*/
func roomFor(ctx context.Context, q *models.Queries, workspaceID ids.UUID) error {
	active, err := q.CountActiveAIConversationAttributes(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("count ai attributes: %w", err)
	}

	if active >= maxActive {
		return ErrLimit
	}

	return nil
}

/*
changed applies an update to an attribute, checking what it changes.
*/
func changed(cur models.AiConversationAttribute, in UpdateInput) (models.AiConversationAttribute, error) {
	next := cur
	if in.Label != nil {
		next.Label = strings.TrimSpace(*in.Label)
	}

	if in.Description != nil {
		next.Description = strings.TrimSpace(*in.Description)
	}

	if in.Options != nil {
		next.Options = trimmed(in.Options)
	}

	if in.Status != nil {
		next.Status = *in.Status
	}

	if next.Status != "active" && next.Status != "archived" {
		return next, ErrStatus
	}

	return next, checkWords(next.Label, next.Description, next.DataType, next.Options)
}

/*
checkNew checks a new attribute.
*/
func checkNew(in CreateInput) error {
	if !keyPattern.MatchString(in.Key) {
		return ErrKey
	}

	if !slices.Contains(aidecision.Types(), in.DataType) {
		return ErrType
	}

	return checkWords(strings.TrimSpace(in.Label), strings.TrimSpace(in.Description), in.DataType, trimmed(in.Options))
}

/*
checkWords checks an attribute's name, description and choices.
*/
func checkWords(label, description, dataType string, options []string) error {
	if n := utf8.RuneCountInString(label); n == 0 || n > maxLabel {
		return ErrLabel
	}

	if utf8.RuneCountInString(description) > maxDescription {
		return ErrDescription
	}

	if dataType != aidecision.TypeChoice {
		if len(options) > 0 {
			return ErrOptions
		}

		return nil
	}

	if len(options) == 0 || len(options) > maxChoices {
		return ErrOptions
	}

	seen := map[string]bool{}
	for _, o := range options {
		if o == "" || utf8.RuneCountInString(o) > maxLabel || seen[o] {
			return ErrOptions
		}

		seen[o] = true
	}

	return nil
}

/*
trimmed trims each choice, an empty list for none.
*/
func trimmed(options []string) []string {
	out := make([]string, 0, len(options))
	for _, o := range options {
		out = append(out, strings.TrimSpace(o))
	}

	return out
}
