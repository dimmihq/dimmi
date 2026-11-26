/*
|--------------------------------------------------------------------------
| Using And Deleting Conversation Attributes
|--------------------------------------------------------------------------
|
| An attribute can be deleted only while nothing uses it: no
| conversation holds a value of it, and no workflow or inbox view reads
| or sets it. Otherwise the refusal names what uses it, and archiving is
| the way to stop it. Usage says the same before anyone asks to delete.
|
*/

package aiattributes

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ErrInUse refuses to delete an attribute something uses; its meta names
how many conversations hold a value, and the workflows and views.
*/
var ErrInUse = fault.New(fault.Conflict, "attribute_in_use",
	"Conversations, workflows or views use this attribute. Archive it instead.")

/*
Named is a workflow or view by its id and name.
*/
type Named struct {
	ID   ids.UUID
	Name string
}

/*
Usage is what uses an attribute.
*/
type Usage struct {
	Conversations int64
	Workflows     []Named
	Views         []Named
}

/*
Unused reports whether nothing uses the attribute.
*/
func (u Usage) Unused() bool {
	return u.Conversations == 0 && len(u.Workflows) == 0 && len(u.Views) == 0
}

/*
Usage says what uses an attribute.
*/
func (a *Attributes) Usage(ctx context.Context, m authz.Membership, id ids.UUID) (Usage, error) {
	if !canManage(m) {
		return Usage{}, exceptions.ErrForbidden
	}

	var out Usage

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		attr, err := find(ctx, q, m.Workspace.ID, id)
		if err != nil {
			return err
		}

		out, err = usageOf(ctx, q, attr)

		return err
	})

	return out, err
}

/*
Delete removes an attribute nothing uses.
*/
func (a *Attributes) Delete(ctx context.Context, m authz.Membership, id ids.UUID) error {
	if !canManage(m) {
		return exceptions.ErrForbidden
	}

	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		attr, err := find(ctx, q, m.Workspace.ID, id)
		if err != nil {
			return err
		}

		used, err := usageOf(ctx, q, attr)
		if err != nil {
			return err
		}

		if !used.Unused() {
			return ErrInUse.With("conversations", used.Conversations).
				With("workflows", names(used.Workflows)).With("views", names(used.Views))
		}

		if _, err := q.DeleteConversationAttribute(ctx, models.DeleteConversationAttributeParams{
			WorkspaceID: m.Workspace.ID, ID: id,
		}); err != nil {
			return fmt.Errorf("delete conversation attribute: %w", err)
		}

		if err := q.ForgetConversationAttributeSources(ctx, models.ForgetConversationAttributeSourcesParams{
			WorkspaceID: m.Workspace.ID, Key: attr.Key,
		}); err != nil {
			return fmt.Errorf("forget conversation attribute: %w", err)
		}

		return nil
	})
}

/*
find returns an attribute of the workspace.
*/
func find(ctx context.Context, q *models.Queries, workspaceID, id ids.UUID) (models.AiConversationAttribute, error) {
	attr, err := q.FindAIConversationAttribute(ctx, models.FindAIConversationAttributeParams{
		WorkspaceID: workspaceID, ID: id,
	})
	if errors.Is(err, database.ErrNotFound) {
		return attr, ErrNotFound
	}

	if err != nil {
		return attr, fmt.Errorf("find conversation attribute: %w", err)
	}

	return attr, nil
}

/*
usageOf finds what uses an attribute: conversations holding a value,
workflows reading it as a field or a variable or setting it, and views
filtering by it.
*/
func usageOf(ctx context.Context, q *models.Queries, attr models.AiConversationAttribute) (Usage, error) {
	ws, field := attr.WorkspaceID, "conversation.ai."+attr.Key

	count, err := q.CountConversationsWithAttribute(ctx, models.CountConversationsWithAttributeParams{
		WorkspaceID: ws, Key: attr.Key,
	})
	if err != nil {
		return Usage{}, fmt.Errorf("count conversations with attribute: %w", err)
	}

	out := Usage{Conversations: count}

	for _, needle := range []string{field + `"`, field + "}"} {
		rows, err := q.ListWorkflowsUsingAttribute(ctx, models.ListWorkflowsUsingAttributeParams{
			WorkspaceID: ws, Field: needle, StepKey: `"key": "` + attr.Key + `"`,
		})
		if err != nil {
			return Usage{}, fmt.Errorf("list workflows using attribute: %w", err)
		}

		for _, r := range rows {
			if !slices.ContainsFunc(out.Workflows, func(n Named) bool { return n.ID == r.ID }) {
				out.Workflows = append(out.Workflows, Named{ID: r.ID, Name: r.Name})
			}
		}
	}

	views, err := q.ListViewsUsingAttribute(ctx, models.ListViewsUsingAttributeParams{WorkspaceID: ws, Field: field + `"`})
	if err != nil {
		return Usage{}, fmt.Errorf("list views using attribute: %w", err)
	}

	for _, v := range views {
		out.Views = append(out.Views, Named{ID: v.ID, Name: v.Name})
	}

	return out, nil
}

func names(list []Named) []string {
	out := make([]string, 0, len(list))
	for _, n := range list {
		out = append(out, n.Name)
	}

	return out
}
