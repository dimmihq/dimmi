/*
|--------------------------------------------------------------------------
| Managing Statuses
|--------------------------------------------------------------------------
|
| The team adds statuses, renames and recolors them, moves them to
| another category, and removes them. A status gets its key from its
| first name and keeps it through renames, so links and the API stay
| stable. Removing a status moves its requests to another one first, so
| no request is ever left without a status, and the last status stays.
|
*/

package feedback

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Failures of managing statuses.
*/
var (
	ErrStatusNotFound = fault.New(fault.NotFound, "feedback_status_not_found",
		"No status of this workspace has this key.")
	ErrStatusNameInvalid = fault.New(fault.Invalid, "name_invalid",
		"Status names are 1 to 40 characters.").OnField("name")
	ErrStatusColorInvalid = fault.New(fault.Invalid, "color_invalid",
		"Colors are hex, like #3b82f6.").OnField("color")
	ErrCategoryInvalid = fault.New(fault.Invalid, "category_invalid",
		"The category is open, planned, in_progress, done or closed.").OnField("category")
	ErrLastStatus = fault.New(fault.Conflict, "last_status",
		"Keep at least one status.")
	ErrMoveTargetInvalid = fault.New(fault.Invalid, "move_to_invalid",
		"Pick another status to move the requests to.").OnField("move_to")
)

/*
StatusChange adds or changes a status; nil leaves a field as it is.
A new status needs a name.
*/
type StatusChange struct {
	Name     *string
	Color    *string
	Category *string
	Roadmap  *bool
}

/*
CreateStatus adds a status after the others.
*/
func (p *Posts) CreateStatus(ctx context.Context, m authz.Membership, in StatusChange) (Status, error) {
	if !m.Can("settings:write") {
		return Status{}, exceptions.ErrForbidden
	}
	s := Status{Color: "#a1a1aa", Category: CategoryOpen, Roadmap: true}
	if err := in.over(&s); err != nil {
		return Status{}, err
	}
	if in.Roadmap == nil {
		s.Roadmap = s.Category != CategoryClosed
	}
	var out Status
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		current, err := ordered(ctx, q, m.Workspace.ID)
		if err != nil {
			return err
		}
		row, err := q.CreateFeedbackStatus(ctx, models.CreateFeedbackStatusParams{
			WorkspaceID: m.Workspace.ID, Key: freeKey(current, s.Name), Name: s.Name, Color: s.Color,
			Category: s.Category, Roadmap: s.Roadmap, Now: p.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("create feature request status: %w", err)
		}
		out = statusOfRow(row)
		return nil
	})
	return out, err
}

/*
UpdateStatus renames, recolors or recategorizes a status.
*/
func (p *Posts) UpdateStatus(ctx context.Context, m authz.Membership, key string, in StatusChange) (Status, error) {
	if !m.Can("settings:write") {
		return Status{}, exceptions.ErrForbidden
	}
	var out Status
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		current, err := ordered(ctx, q, m.Workspace.ID)
		if err != nil {
			return err
		}
		s, ok := statusIn(current, key)
		if !ok {
			return ErrStatusNotFound
		}
		if err := in.over(&s); err != nil {
			return err
		}
		row, err := q.UpdateFeedbackStatus(ctx, models.UpdateFeedbackStatusParams{
			Name: s.Name, Color: s.Color, Category: s.Category, Roadmap: s.Roadmap, Now: p.Clock.Now(),
			WorkspaceID: m.Workspace.ID, Key: key,
		})
		if err != nil {
			return fmt.Errorf("update feature request status: %w", err)
		}
		out = statusOfRow(row)
		return nil
	})
	return out, err
}

/*
DeleteStatus removes a status, moving its requests to the status named
by moveTo, or to the first other status of the same category, or else
the first other status.
*/
func (p *Posts) DeleteStatus(ctx context.Context, m authz.Membership, key, moveTo string) error {
	if !m.Can("settings:write") {
		return exceptions.ErrForbidden
	}
	return p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		current, err := ordered(ctx, q, m.Workspace.ID)
		if err != nil {
			return err
		}
		gone, ok := statusIn(current, key)
		if !ok {
			return ErrStatusNotFound
		}
		if len(current) == 1 {
			return ErrLastStatus
		}
		target, err := moveTarget(current, gone, moveTo)
		if err != nil {
			return err
		}
		now := p.Clock.Now()
		if err := q.MoveFeedbackPostsToStatus(ctx, models.MoveFeedbackPostsToStatusParams{
			Target: target, Now: now, WorkspaceID: m.Workspace.ID, Source: key,
		}); err != nil {
			return fmt.Errorf("move feature requests to another status: %w", err)
		}
		if err := q.DeleteFeedbackStatus(ctx, models.DeleteFeedbackStatusParams{
			WorkspaceID: m.Workspace.ID, Key: key,
		}); err != nil {
			return fmt.Errorf("delete feature request status: %w", err)
		}
		return nil
	})
}

func moveTarget(current []Status, gone Status, moveTo string) (string, error) {
	if moveTo != "" {
		if _, ok := statusIn(current, moveTo); !ok || moveTo == gone.Key {
			return "", ErrMoveTargetInvalid
		}
		return moveTo, nil
	}
	others := slices.DeleteFunc(slices.Clone(current), func(s Status) bool { return s.Key == gone.Key })
	for _, s := range others {
		if s.Category == gone.Category {
			return s.Key, nil
		}
	}
	return others[0].Key, nil
}

/*
over writes the change over a status, checking each field it names.
*/
func (c StatusChange) over(s *Status) error {
	if c.Name != nil {
		name := strings.TrimSpace(*c.Name)
		if name == "" || len([]rune(name)) > 40 {
			return ErrStatusNameInvalid
		}
		s.Name = name
	}
	if s.Name == "" {
		return ErrStatusNameInvalid
	}
	if c.Color != nil {
		if !hexColor().MatchString(*c.Color) {
			return ErrStatusColorInvalid
		}
		s.Color = strings.ToLower(*c.Color)
	}
	if c.Category != nil {
		if !slices.Contains(Categories(), *c.Category) {
			return ErrCategoryInvalid
		}
		s.Category = *c.Category
	}
	if c.Roadmap != nil {
		s.Roadmap = *c.Roadmap
	}
	return nil
}

func hexColor() *regexp.Regexp {
	return regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
}

/*
freeKey makes a key from the name that no status of the workspace
holds yet.
*/
func freeKey(current []Status, name string) string {
	base := strings.ReplaceAll(helpcenters.Slugify(name, 32), "-", "_")
	if base == "" {
		base = "status"
	}
	key := base
	for n := 2; ; n++ {
		if _, taken := statusIn(current, key); !taken {
			return key
		}
		key = base + "_" + strconv.Itoa(n)
	}
}
