/*
|--------------------------------------------------------------------------
| Create User Command
|--------------------------------------------------------------------------
|
| The operator of a server adds people from the command line: the very
| first owner of a fresh install, or someone added to a workspace
| without mail. Whoever runs the server vouches for the address, so the
| account starts verified. On an empty install the person becomes the
| owner of a new workspace, the same way the setup page would do it, and
| only one of the two can win.
|
*/

package commands

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/support/str"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/users"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The defaults of a new user: the name of the first workspace and the
length of a generated password.
*/
const (
	firstWorkspaceName     = "My workspace"
	generatedPasswordChars = 20
)

/*
User creation failures.

ErrPasswordMissing means neither a password nor --generate-password was
given. ErrUnknownRole means --role named no role.
*/
var (
	ErrPasswordMissing = errors.New("give --password or --generate-password")
	ErrUnknownRole     = errors.New("the role must be owner, admin or member")
)

/*
UserCreateCommand creates an account, and its first workspace on an
empty install.
*/
type UserCreateCommand struct {
	App *app.App
}

/*
Name returns user:create.
*/
func (c *UserCreateCommand) Name() string {
	return "user:create"
}

/*
Description says what the command does.
*/
func (c *UserCreateCommand) Description() string {
	return "Create a verified user, and the first workspace on an empty server"
}

/*
Flags declares the account, its password and where it joins.
*/
func (c *UserCreateCommand) Flags(flags *console.Flags) {
	flags.String("email", "", "the email of the account")
	flags.String("name", "", "the full name of the person")
	flags.String("password", "", "the password to set")
	flags.Bool("generate-password", "make up a password and print it")
	flags.String("workspace", "", "the name of the workspace to create")
	flags.String("workspace-slug", "", "the address of an existing workspace to join")
	flags.String("role", "", "owner, admin or member in that workspace")
}

/*
Handle creates the account and says where it landed.
*/
func (c *UserCreateCommand) Handle(
	ctx context.Context, _ *foundation.Application, in *console.Input, out *console.Output,
) error {
	password, generated, err := c.password(in)
	if err != nil {
		return err
	}

	request := newUserRequest{
		email: strings.TrimSpace(in.String("email")), name: strings.TrimSpace(in.String("name")),
		password: password, workspace: strings.TrimSpace(in.String("workspace")),
		slug: strings.TrimSpace(in.String("workspace-slug")), role: in.String("role"),
	}

	if err := c.create(ctx, request, out); err != nil {
		return err
	}

	if generated {
		out.Line("Password: %s", password)
	}

	return nil
}

/*
newUserRequest is what the operator asked for.
*/
type newUserRequest struct {
	email, name, password, workspace, slug, role string
	joinAs                                       enums.Role
}

/*
password returns the password asked for, or a generated one and that it
was generated.
*/
func (c *UserCreateCommand) password(in *console.Input) (string, bool, error) {
	if in.Bool("generate-password") {
		return str.Random(generatedPasswordChars), true, nil
	}

	if in.String("password") == "" {
		return "", false, ErrPasswordMissing
	}

	return in.String("password"), false, nil
}

/*
create makes the account where the request says: as the first user, in
an existing workspace, or on its own.
*/
func (c *UserCreateCommand) create(ctx context.Context, r newUserRequest, out *console.Output) error {
	role, err := roleOf(r)
	if err != nil {
		return err
	}

	r.joinAs = role

	existing, err := models.New(c.App.DB).FindUserByEmail(ctx, strings.ToLower(r.email))
	if err == nil {
		return c.join(ctx, existing, r, out)
	}

	if !errors.Is(err, database.ErrNotFound) {
		return fmt.Errorf("find user: %w", err)
	}

	if err := c.checkPassword(r.password); err != nil {
		return err
	}

	hash, err := c.App.Hash.Make(ctx, r.password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	first, err := c.App.Actions.CreateFirstUser.Handle(ctx, users.CreateFirstUserInput{
		Name: r.name, Email: r.email, PasswordHash: &hash, WorkspaceName: workspaceName(r.workspace), Via: "cli",
	})
	if err == nil {
		out.Info("Created %s, owner of %s (/w/%s).", first.User.Email, first.Workspace.Workspace.Name,
			first.Workspace.Workspace.Slug)

		return nil
	}

	if !errors.Is(err, exceptions.ErrSetupCompleted) {
		return err
	}

	return c.addUser(ctx, r, &hash, out)
}

/*
checkPassword refuses a password shorter than the server allows.
*/
func (c *UserCreateCommand) checkPassword(password string) error {
	if len([]rune(password)) < c.App.Config.Auth.Passwords.MinLength {
		return exceptions.ErrPasswordTooShort
	}

	return nil
}

/*
addUser creates the account on a server that already has people, and
puts it in the workspace asked for.
*/
func (c *UserCreateCommand) addUser(ctx context.Context, r newUserRequest, hash *string, out *console.Output) error {
	user, err := c.App.Actions.CreateUser.Handle(ctx, users.CreateUserInput{
		Name: r.name, Email: r.email, PasswordHash: hash, Verified: true, CreatedVia: "cli",
	})
	if err != nil {
		return err
	}

	if r.slug == "" {
		out.Info("Created %s. Invite them to a workspace or pass --workspace-slug.", user.Email)

		return nil
	}

	return c.join(ctx, user, r, out)
}

/*
join puts the account in the workspace the request names.
*/
func (c *UserCreateCommand) join(ctx context.Context, user models.User, r newUserRequest, out *console.Output) error {
	if r.slug == "" {
		return exceptions.ErrEmailTaken
	}

	member, err := c.App.Actions.OperatorAccounts.Join(ctx, user, r.slug, r.joinAs)
	if err != nil {
		return err
	}

	out.Info("%s is now %s of /w/%s.", user.Email, member.Role, r.slug)

	return nil
}

/*
roleOf reads the role asked for: member by default. A role only means
something with a workspace to join.
*/
func roleOf(r newUserRequest) (enums.Role, error) {
	if r.role == "" {
		return enums.RoleMember, nil
	}

	role := enums.Role(strings.ToLower(r.role))
	if !slices.Contains(enums.Roles(), role) || r.slug == "" {
		return "", ErrUnknownRole
	}

	return role, nil
}

/*
workspaceName is the name of the first workspace, the one asked for or
the default.
*/
func workspaceName(asked string) string {
	if asked == "" {
		return firstWorkspaceName
	}

	return asked
}
