/*
|--------------------------------------------------------------------------
| Create Workspace
|--------------------------------------------------------------------------
|
| A workspace never exists without its organization and its first owner:
| the three rows are written in one transaction, organization first, so
| no failure can leave a workspace nobody owns or an organization with
| nothing in it. Every workspace gets an organization of its own for
| now; the organization stays invisible to clients.
|
| The slug comes from the name: ASCII letters, digits and dashes, at most
| 40 characters. A taken slug gets -2, -3 and so on, and after -99 a
| random suffix. Slugs that would read as part of the product, such as
| admin or login, are never handed out, and neither is anything shorter
| than three characters or a slug another workspace gave up recently.
|
| Once the workspace and its owner exist, every registered initializer
| adds its defaults in the same transaction, so any failure among them
| leaves nothing behind.
|
*/

package workspaces

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/support/str"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
CreateWorkspaceInput is the workspace to create and its first owner.
JoinedVia is how the owner came to be a member, such as created.
*/
type CreateWorkspaceInput struct {
	Owner     models.User
	Name      string
	JoinedVia string
}

/*
CreatedWorkspace is the new workspace and its owner's membership.
*/
type CreatedWorkspace struct {
	Workspace models.Workspace
	Owner     models.WorkspaceMember
}

/*
CreateWorkspace creates a workspace with its organization and owner.

Rules:
  - The organization is created first, named after the workspace.
  - The owner joins as an active member with the owner role.
  - The initializers run after the owner joins, in the transaction.

Events: [appevents.WorkspaceCreated], [appevents.MemberJoined].

Errors: a wrapped database error.
*/
type CreateWorkspace struct {
	DB           *database.DB
	Events       *events.Dispatcher
	Clock        clock.Clock
	Initializers *Initializers
}

/*
Handle creates the workspace, inside the transaction in the context
when there is one.
*/
func (a *CreateWorkspace) Handle(ctx context.Context, in CreateWorkspaceInput) (CreatedWorkspace, error) {
	now := a.Clock.Now()
	id := ids.NewV7At(now)
	name := str.Clean(in.Name)

	var created CreatedWorkspace

	err := a.DB.Transaction(tenancy.With(ctx, id.String()), func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		org, err := q.CreateOrganization(ctx, models.CreateOrganizationParams{
			Name: name, CreatedByUserID: &in.Owner.ID, Now: now,
		})
		if err != nil {
			return fmt.Errorf("create organization: %w", err)
		}

		slug, err := a.freeSlug(ctx, q, name, now)
		if err != nil {
			return err
		}

		created.Workspace, err = q.CreateWorkspace(ctx, models.CreateWorkspaceParams{
			ID: id, OrganizationID: org.ID, Name: name, Slug: slug, Timezone: "UTC", Locale: "en",
			CreatedByUserID: &in.Owner.ID, Now: now,
		})
		if err != nil {
			return fmt.Errorf("create workspace: %w", err)
		}

		created.Owner, err = q.CreateWorkspaceMember(ctx, models.CreateWorkspaceMemberParams{
			WorkspaceID: id, UserID: in.Owner.ID, Role: string(enums.RoleOwner), JoinedVia: &in.JoinedVia, Now: now,
		})
		if err != nil {
			return fmt.Errorf("add workspace owner: %w", err)
		}

		if err := a.Initializers.Run(ctx, tx, created.Workspace); err != nil {
			return err
		}

		return a.announce(ctx, tx, created, in.JoinedVia)
	})

	return created, err
}

/*
announce dispatches the events of a new workspace and its owner.
*/
func (a *CreateWorkspace) announce(ctx context.Context, tx *database.Tx, c CreatedWorkspace, joinedVia string) error {
	created := appevents.WorkspaceCreated{WorkspaceID: c.Workspace.ID, CreatedBy: c.Owner.UserID}

	err := a.Events.Dispatch(ctx, tx, created)
	if err != nil {
		return err
	}

	return a.Events.Dispatch(ctx, tx, appevents.MemberJoined{
		WorkspaceID: c.Workspace.ID, MemberID: c.Owner.ID, UserID: c.Owner.UserID, JoinedVia: joinedVia,
	})
}

/*
freeSlug returns the first slug for the name no workspace holds.
*/
func (a *CreateWorkspace) freeSlug(ctx context.Context, q *models.Queries, name string, now time.Time) (string, error) {
	base := baseSlug(name)

	for n := 1; n <= slugMaxSuffix; n++ {
		candidate := withSuffix(base, n)

		taken, err := slugTaken(ctx, q, models.CountWorkspacesWithSlugParams{
			WorkspaceID: ids.Nil(), Slug: candidate, Now: now,
		})
		if err != nil {
			return "", err
		}

		if !taken {
			return candidate, nil
		}
	}

	return withSuffix(base, 0), nil
}

/*
baseSlug turns a name into a slug that is long enough and not reserved.
*/
func baseSlug(name string) string {
	slug := str.Slug(name)
	if len(slug) > slugMaxLength {
		slug = slug[:slugMaxLength]
	}

	slug = strings.Trim(slug, "-")

	if len(slug) < slugMinLength || reservedSlug(slug) {
		slug = strings.TrimPrefix(slug+"-workspace", "-")
	}

	return slug
}

/*
withSuffix returns the slug for the nth try: the base itself first,
then -2 to -99, and for zero a random suffix. The result never passes
the length limit.
*/
func withSuffix(base string, n int) string {
	suffix := ""

	switch {
	case n == 0:
		suffix = "-" + strings.ToLower(str.Random(4))
	case n > 1:
		suffix = "-" + strconv.Itoa(n)
	}

	room := slugMaxLength - len(suffix)
	if len(base) > room {
		base = strings.TrimRight(base[:room], "-")
	}

	return base + suffix
}
