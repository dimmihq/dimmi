/*
|--------------------------------------------------------------------------
| Saving Views
|--------------------------------------------------------------------------
|
| Anyone who works conversations may keep personal views; shared views
| take views:manage_shared. A personal view is its owner's alone to
| change. Every save checks the conditions the way a list will run them,
| so a view that saves always opens. A workspace holds 200 shared views
| and each member 50 personal ones.
|
*/

package conversationviews

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/querybuilder"
)

/*
The scopes of a view, and how many views each may hold.
*/
const (
	ScopeShared   = "shared"
	ScopePersonal = "personal"

	maxShared   = 200
	maxPersonal = 50
)

/*
Icons returns the icons a view may show.
*/
func Icons() []string {
	return []string{
		"inbox", "star", "flag", "fire", "time", "user", "team", "price-tag", "mail", "chat", "bug", "money",
		"heart", "alarm", "rocket", "shield",
	}
}

/*
Fields are what a create or update sets; nil leaves a field as it is,
and an empty Icon, Sort or StatusFilter clears it.
*/
type Fields struct {
	Name         *string
	Icon         *string
	Definition   json.RawMessage
	Sort         *string
	StatusFilter *string
}

/*
CreateInput is who saves which view, shared or personal.
*/
type CreateInput struct {
	Membership authz.Membership
	Scope      string
	Fields     Fields
}

/*
Saver holds what every write needs.
*/
type Saver struct {
	DB     *database.DB
	IDs    *ids.Registry
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
CreateView saves a new view.

Rules:
  - The caller needs conversations:write, and views:manage_shared for a
    shared view.
  - The name is 1 to 60 characters, unique in its scope and owner.
  - The conditions compile; the sort and status are ones a list has.
  - At most 200 shared views, and 50 personal per member.

Events: [appevents.ConversationViewChanged].

Errors: [exceptions.ErrForbidden], [exceptions.ErrInvalidDefinition],
[exceptions.ErrViewLimitReached], [exceptions.ErrViewNameTaken], or a
wrapped database error.
*/
type CreateView struct {
	Saver
}

/*
Handle saves the view and returns it.
*/
func (a *CreateView) Handle(ctx context.Context, in CreateInput) (View, error) {
	if err := canChange(in.Membership, in.Scope, nil); err != nil {
		return View{}, err
	}

	if in.Fields.Name == nil || in.Fields.Definition == nil {
		return View{}, exceptions.ErrInvalidDefinition.With("path", "definition")
	}

	name, err := a.validate(in.Membership, in.Fields)
	if err != nil {
		return View{}, err
	}

	var out View

	err = a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		params := createParams(in, name, a.Clock)

		if err := checkLimit(ctx, q, params); err != nil {
			return err
		}

		row, err := q.CreateConversationView(ctx, params)
		if database.IsUniqueViolation(err, "") {
			return exceptions.ErrViewNameTaken
		}

		if err != nil {
			return fmt.Errorf("create conversation view: %w", err)
		}

		out, err = a.finish(ctx, tx, saved{membership: in.Membership, row: row, action: "created"})

		return err
	})

	return out, err
}

/*
createParams turns the input into the new row.
*/
func createParams(in CreateInput, name string, c clock.Clock) models.CreateConversationViewParams {
	params := models.CreateConversationViewParams{
		WorkspaceID: in.Membership.Workspace.ID, Name: name, Scope: in.Scope, Definition: in.Fields.Definition,
		Icon: blankNil(in.Fields.Icon), Sort: blankNil(in.Fields.Sort), StatusFilter: blankNil(in.Fields.StatusFilter),
		CreatedByMemberID: new(in.Membership.Member.ID), Now: c.Now(),
	}

	if in.Scope == ScopePersonal {
		params.OwnerMemberID = new(in.Membership.Member.ID)
	}

	return params
}

/*
checkLimit refuses a view past its scope's limit.
*/
func checkLimit(ctx context.Context, q *models.Queries, p models.CreateConversationViewParams) error {
	count, err := q.CountConversationViews(ctx, models.CountConversationViewsParams{
		WorkspaceID: p.WorkspaceID, Scope: p.Scope, OwnerMemberID: p.OwnerMemberID,
	})
	if err != nil {
		return fmt.Errorf("count conversation views: %w", err)
	}

	limit := int64(maxShared)
	if p.Scope == ScopePersonal {
		limit = maxPersonal
	}

	if count >= limit {
		return exceptions.ErrViewLimitReached
	}

	return nil
}

/*
canChange refuses a caller who may not save views of the scope: anyone
working conversations for their own, views:manage_shared for shared
ones. The owner given is that of the personal view being changed, nil
when creating.
*/
func canChange(m authz.Membership, scope string, owner *ids.UUID) error {
	if err := conversations.CanWrite(m); err != nil {
		return err
	}

	switch scope {
	case ScopeShared:
		if !m.Can("views:manage_shared") {
			return exceptions.ErrForbidden
		}
	case ScopePersonal:
		if owner != nil && *owner != m.Member.ID {
			return exceptions.ErrNotFound
		}
	default:
		return exceptions.ErrInvalidDefinition.With("path", "scope")
	}

	return nil
}

/*
validate checks the fields a save sets and returns the cleaned name.
*/
func (s Saver) validate(m authz.Membership, f Fields) (string, error) {
	var name string

	if f.Name != nil {
		name = strings.TrimSpace(*f.Name)
		if name == "" || len([]rune(name)) > 60 {
			return "", exceptions.ErrInvalidDefinition.With("path", "name")
		}
	}

	if err := checkOptions(f); err != nil {
		return "", err
	}

	if f.Definition == nil {
		return name, nil
	}

	d, err := conversations.ReadDefinition(f.Definition)
	if err != nil {
		return "", err
	}

	if _, err := conversations.NewCompiler(m, s.IDs, s.Clock.Now()).Compile(d); err != nil {
		return "", conversations.CompileError(err)
	}

	return name, nil
}

/*
saved is a view as a write left it: who wrote, the row, and created,
updated or deleted.
*/
type saved struct {
	membership authz.Membership
	row        models.ConversationView
	action     string
}

/*
checkOptions refuses an icon, sort or status no view may have.
*/
func checkOptions(f Fields) error {
	if f.Icon != nil && *f.Icon != "" && !slices.Contains(Icons(), *f.Icon) {
		return exceptions.ErrInvalidDefinition.With("path", "icon")
	}

	if f.Sort != nil && *f.Sort != "" && !slices.Contains(querybuilder.Sorts(), querybuilder.Sort(*f.Sort)) {
		return exceptions.ErrInvalidDefinition.With("path", "sort")
	}

	if f.StatusFilter != nil && *f.StatusFilter != "" &&
		!slices.Contains([]string{"open", "snoozed", "closed", "any"}, *f.StatusFilter) {
		return exceptions.ErrInvalidDefinition.With("path", "status_filter")
	}

	return nil
}

/*
finish announces the change and returns the view with its warnings.
*/
func (s Saver) finish(ctx context.Context, tx *database.Tx, done saved) (View, error) {
	m, row := done.membership, done.row

	err := s.Events.Dispatch(ctx, tx, appevents.ConversationViewChanged{
		Base: appevents.Base{
			ID: ids.New(), WorkspaceID: m.Workspace.ID, OccurredAt: s.Clock.Now(),
			Actor: appevents.Actor{Type: "member", ID: m.Member.ID},
		},
		ViewID: row.ID, Action: done.action, OwnerMemberID: row.OwnerMemberID,
	})
	if err != nil {
		return View{}, err
	}

	views, err := warner{q: models.New(tx), ids: s.IDs}.views(ctx, m, []models.ConversationView{row})
	if err != nil {
		return View{}, err
	}

	return views[0], nil
}

/*
blankNil returns nil for a missing or empty value.
*/
func blankNil(value *string) *string {
	if value == nil || *value == "" {
		return nil
	}

	return value
}
