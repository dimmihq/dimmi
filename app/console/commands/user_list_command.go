/*
|--------------------------------------------------------------------------
| List Users Command
|--------------------------------------------------------------------------
|
| The operator of a server sees who has an account: whether each email
| is proven, which second factors protect it, and the workspaces it
| works in. Deleted accounts are left out. With --json the same list
| goes to scripts.
|
*/

package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/foundation"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/models"
)

/*
UserListCommand lists the server's accounts.
*/
type UserListCommand struct {
	App *app.App
}

/*
listedUser is one account as the list shows it.
*/
type listedUser struct {
	ID         string   `json:"id"`
	Email      string   `json:"email"`
	Verified   bool     `json:"verified"`
	TwoFactor  []string `json:"two_factor"`
	Workspaces []string `json:"workspaces"`
}

/*
Name returns user:list.
*/
func (c *UserListCommand) Name() string {
	return "user:list"
}

/*
Description says what the command does.
*/
func (c *UserListCommand) Description() string {
	return "List the server's users, their second factors and workspaces"
}

/*
Flags declares the JSON switch.
*/
func (c *UserListCommand) Flags(flags *console.Flags) {
	flags.Bool("json", "print the list as JSON")
}

/*
Handle prints the accounts, as a table or as JSON.
*/
func (c *UserListCommand) Handle(
	ctx context.Context, _ *foundation.Application, in *console.Input, out *console.Output,
) error {
	listed, err := c.list(ctx)
	if err != nil {
		return err
	}

	if in.Bool("json") {
		raw, err := json.Marshal(listed)
		if err != nil {
			return fmt.Errorf("encode users: %w", err)
		}

		out.Line("%s", raw)

		return nil
	}

	rows := make([][]string, 0, len(listed))
	for _, u := range listed {
		rows = append(rows, []string{
			u.ID, u.Email, strconv.FormatBool(u.Verified),
			strings.Join(u.TwoFactor, ","), strings.Join(u.Workspaces, ","),
		})
	}

	out.Table([]string{"id", "email", "verified", "2fa", "workspaces"}, rows)

	return nil
}

/*
list reads every live account with its workspaces.
*/
func (c *UserListCommand) list(ctx context.Context) ([]listedUser, error) {
	q := models.New(c.App.DB)

	rows, err := q.ListLiveUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}

	out := make([]listedUser, 0, len(rows))

	for _, row := range rows {
		memberships, err := q.ListUserWorkspaces(ctx, row.User.ID)
		if err != nil {
			return nil, fmt.Errorf("list workspaces of %s: %w", row.User.Email, err)
		}

		slugs := make([]string, 0, len(memberships))
		for _, m := range memberships {
			slugs = append(slugs, m.WorkspaceSlug)
		}

		out = append(out, listedUser{
			ID: models.PublicID(models.PrefixUser, row.User.ID), Email: row.User.Email,
			Verified: row.User.IsVerified(), TwoFactor: row.FactorTypes, Workspaces: slugs,
		})
	}

	return out, nil
}
