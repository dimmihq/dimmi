//go:build dev

/*
|--------------------------------------------------------------------------
| Imported Teammates Demo
|--------------------------------------------------------------------------
|
| Brings a handful of made-up teammates of another tool into a
| development workspace through the real import action, so the imported
| teammates of Settings → Members can be seen and tried: each one waits,
| disabled, for an admin. Some of them can be marked as having signed up
| already, which they do with the demo password, so both states show.
| It writes only to the workspace it is given and never to a release
| build.
|
*/

package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/importedteammates"
	"github.com/dimmihq/dimmi/app/demo/stowline"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
importDemoPerson is one made-up teammate of the other tool.
*/
type importDemoPerson struct {
	name string
	role enums.Role
}

/*
importDemoPeople returns the teammates the demo brings over.
*/
func importDemoPeople() []importDemoPerson {
	return []importDemoPerson{
		{"Amara Okafor", enums.RoleAdmin},
		{"Lucas Moreau", enums.RoleMember},
		{"Priya Raman", enums.RoleMember},
		{"Tomás Herrera", enums.RoleMember},
		{"Yuki Tanaka", enums.RoleMember},
		{"Sven Lindqvist", enums.RoleAdmin},
		{"Zainab Bello", enums.RoleMember},
		{"Mateo Rossi", enums.RoleMember},
	}
}

/*
importDemoAddress is the address of a demo teammate at the domain: their first
name, lower case, with its accents left out.
*/
func importDemoAddress(name, domain string) string {
	plain := strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u")

	return plain.Replace(strings.ToLower(strings.Fields(name)[0])) + "@" + domain
}

/*
TeammatesImportDemoCommand brings made-up teammates of another tool into
a workspace.
*/
type TeammatesImportDemoCommand struct {
	App *app.App
}

/*
Name returns teammates:import-demo.
*/
func (c TeammatesImportDemoCommand) Name() string {
	return "teammates:import-demo"
}

/*
Description says what the command does.
*/
func (c TeammatesImportDemoCommand) Description() string {
	return "Import made-up teammates of another tool into a development workspace, disabled until enabled"
}

/*
Flags name the workspace, the tool the teammates come from, how many to
bring and how many of them have signed up already.
*/
func (c TeammatesImportDemoCommand) Flags(flags *console.Flags) {
	flags.String("workspace", "", "the workspace's public id, ws_...")
	flags.String("source", "intercom", "the tool the teammates come from")
	flags.String("domain", "example.com", "the email domain of the teammates")
	flags.Int("count", 6, "how many teammates to import, at most 8")
	flags.Int("signed-up", 2, "how many of them have signed up and wait")
}

/*
Handle imports the teammates.
*/
func (c TeammatesImportDemoCommand) Handle(
	ctx context.Context, _ *foundation.Application, in *console.Input, out *console.Output,
) error {
	workspace, err := c.App.IDs.Decode(string(models.PrefixWorkspace), in.String("workspace"))
	if err != nil {
		return fmt.Errorf("--workspace: %w", err)
	}

	ctx = tenancy.With(ctx, workspace.String())

	teams, err := c.teams(ctx, workspace)
	if err != nil {
		return err
	}

	people := importDemoPeople()
	count := min(max(in.Int("count"), 0), len(people))
	signedUp := min(max(in.Int("signed-up"), 0), count)

	for i, person := range people[:count] {
		email := importDemoAddress(person.name, in.String("domain"))

		imported, err := c.App.Actions.CreateImportedTeammate.Handle(ctx, importedteammates.CreateImportedTeammateInput{
			WorkspaceID: workspace, Email: email, Name: person.name, Role: person.role,
			TeamIDs: teams, Source: in.String("source"),
		})
		if err != nil {
			return fmt.Errorf("import %s: %w", email, err)
		}

		state := "not signed up"
		if i < signedUp && imported.User.IsPlaceholder() {
			if err := c.signUp(ctx, workspace, imported.User); err != nil {
				return err
			}

			state = "signed up, password " + stowline.DefaultPassword
		}

		out.Line("%s <%s> %s, %s", person.name, email, person.role, state)
	}

	return nil
}

/*
teams returns the workspace's first team, to put the teammates in, or
none.
*/
func (c TeammatesImportDemoCommand) teams(ctx context.Context, workspace ids.UUID) ([]ids.UUID, error) {
	var found []ids.UUID

	err := c.App.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var team ids.UUID

		err := tx.QueryRow(ctx, `SELECT id FROM teams WHERE workspace_id = $1 AND deleted_at IS NULL
			ORDER BY created_at LIMIT 1`, workspace).Scan(&team)
		if err == nil {
			found = append(found, team)
		}

		return nil
	})

	return found, err
}

/*
signUp makes the placeholder the teammate's own, as signing up and
proving the address would, and records that they wait.
*/
func (c TeammatesImportDemoCommand) signUp(ctx context.Context, workspace ids.UUID, user models.User) error {
	hash, err := c.App.Hash.Make(ctx, stowline.DefaultPassword)
	if err != nil {
		return fmt.Errorf("hash demo password: %w", err)
	}

	now := c.App.Clock.Now()

	return c.App.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		_, err := q.AdoptPlaceholderUser(ctx, models.AdoptPlaceholderUserParams{
			Name: user.Name, Email: user.Email, PasswordHash: &hash, EmailVerifiedAt: &now, Now: now,
		})
		if err != nil {
			return fmt.Errorf("sign up %s: %w", user.Email, err)
		}

		_, err = q.MarkImportArrived(ctx, models.MarkImportArrivedParams{
			WorkspaceID: workspace, UserID: user.ID, Now: now,
		})

		return err
	})
}
