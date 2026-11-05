/*
|--------------------------------------------------------------------------
| Token Create Command
|--------------------------------------------------------------------------
|
| dimmi token:create mints a personal access token for a member without
| a browser, so a CI job or Terraform can bootstrap a fresh server. The
| token is minted exactly as the member would mint it: the same ability
| groups, never more than their role, the same limits, and the owner is
| mailed. The secret is printed once and never again.
|
*/

package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/foundation"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/tokens"
	"github.com/dimmihq/dimmi/app/actions/workspaces"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ErrTokenFlagMissing refuses a token without an email, a workspace, a
name or abilities. ErrExpiryInvalid refuses an expiry that is neither
a number of days, such as 90d, nor a duration, such as 720h.
*/
var (
	ErrTokenFlagMissing = errors.New("--email, --workspace-slug, --name and --abilities are required")
	ErrExpiryInvalid    = errors.New("--expires takes days, such as 90d, or a duration, such as 720h")
)

/*
mintedToken is a new token as the JSON output shows it.
*/
type mintedToken struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Abilities []string   `json:"abilities"`
	ExpiresAt *time.Time `json:"expires_at"`
	Token     string     `json:"token"`
}

/*
TokenCreateCommand mints a personal access token for a member.
*/
type TokenCreateCommand struct {
	App *app.App
}

var _ foundation.Command = TokenCreateCommand{}

/*
Name returns token:create.
*/
func (TokenCreateCommand) Name() string {
	return "token:create"
}

/*
Description says what the command does.
*/
func (TokenCreateCommand) Description() string {
	return "Mint a personal access token for a member and print it once"
}

/*
Flags declares the member, the workspace, the token's name, abilities
and expiry, and the JSON switch.
*/
func (TokenCreateCommand) Flags(flags *console.Flags) {
	flags.String("email", "", "the email of the member the token acts as")
	flags.String("workspace-slug", "", "the workspace the token works in")
	flags.String("name", "", "what the token is for")
	flags.String("abilities", "", "comma-separated ability groups, or * for all the member's role allows")
	flags.String("expires", "", "when the token expires, such as 90d; never when left out")
	flags.Bool("json", "print the token as JSON")
}

/*
Handle mints the token and prints its secret.
*/
func (c TokenCreateCommand) Handle(
	ctx context.Context, _ *foundation.Application, in *console.Input, out *console.Output,
) error {
	email := strings.ToLower(strings.TrimSpace(in.String("email")))
	abilities := splitAbilities(in.String("abilities"))

	if email == "" || in.String("workspace-slug") == "" || in.String("name") == "" || len(abilities) == 0 {
		return ErrTokenFlagMissing
	}

	expiresAt, err := c.expiry(in.String("expires"))
	if err != nil {
		return err
	}

	user, membership, err := c.member(ctx, email, in.String("workspace-slug"))
	if err != nil {
		return err
	}

	made, err := c.App.Actions.CreatePersonalAccessToken.Handle(ctx, tokens.CreatePersonalAccessTokenInput{
		User: user, Membership: membership, Name: in.String("name"), Abilities: abilities, ExpiresAt: expiresAt,
		Client: appauth.Client{UserAgent: "cli"},
	})
	if err != nil {
		return err
	}

	if in.Bool("json") {
		return printTokenJSON(out, made)
	}

	out.Info("Token %q created. Copy it now; it is not shown again.", made.Token.Name)
	out.Line("%s", made.Plain)

	return nil
}

/*
member finds the account with the email and its active membership of
the workspace with the slug, the way the app resolves a signed-in
user's workspace.
*/
func (c TokenCreateCommand) member(ctx context.Context, email, slug string) (models.User, authz.Membership, error) {
	q := models.New(c.App.DB)

	user, err := q.FindUserByEmail(ctx, email)
	if errors.Is(err, database.ErrNotFound) {
		return user, authz.Membership{}, fmt.Errorf("%w: %s", ErrUserNotFound, email)
	}

	if err != nil {
		return user, authz.Membership{}, fmt.Errorf("find user to mint token: %w", err)
	}

	w, err := q.FindWorkspaceBySlug(ctx, slug)
	if errors.Is(err, database.ErrNotFound) {
		return user, authz.Membership{}, exceptions.ErrWorkspaceNotFound
	}

	if err != nil {
		return user, authz.Membership{}, fmt.Errorf("find workspace to mint token: %w", err)
	}

	membership, err := c.App.Actions.ResolveWorkspace.Handle(ctx, workspaces.ResolveWorkspaceInput{
		Principal: &appauth.Principal{User: user, Kind: appauth.KindSession, Granted: []string{appauth.AllAbilities}},
		Named:     models.PublicID(models.PrefixWorkspace, w.ID), Writes: true,
	})

	return user, membership, err
}

/*
expiry reads the expiry flag: a number of days such as 90d, or a Go
duration such as 720h, from now. Empty means the token never expires.
*/
func (c TokenCreateCommand) expiry(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}

	lifetime, err := time.ParseDuration(value)
	if days, found := strings.CutSuffix(value, "d"); found {
		count, convErr := strconv.Atoi(days)
		lifetime, err = time.Duration(count)*24*time.Hour, convErr
	}

	if err != nil || lifetime <= 0 {
		return nil, fmt.Errorf("%w: %q", ErrExpiryInvalid, value)
	}

	at := c.App.Clock.Now().Add(lifetime)

	return &at, nil
}

/*
splitAbilities returns the comma-separated abilities, trimmed, without
empty ones.
*/
func splitAbilities(value string) []string {
	var abilities []string

	for ability := range strings.SplitSeq(value, ",") {
		if trimmed := strings.TrimSpace(ability); trimmed != "" {
			abilities = append(abilities, trimmed)
		}
	}

	return abilities
}

/*
printTokenJSON writes the new token, its secret included, as one JSON
object.
*/
func printTokenJSON(out *console.Output, made tokens.NewToken) error {
	encoded, err := json.Marshal(mintedToken{
		ID: models.PublicID(models.PrefixPersonalToken, made.Token.ID), Name: made.Token.Name,
		Abilities: made.Token.Abilities, ExpiresAt: made.Token.ExpiresAt, Token: made.Plain,
	})
	if err != nil {
		return fmt.Errorf("encode token: %w", err)
	}

	out.Line("%s", encoded)

	return nil
}
