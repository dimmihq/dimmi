//go:build dev

/*
|--------------------------------------------------------------------------
| Demo AI Connection
|--------------------------------------------------------------------------
|
| dimmi ai:demo-connection adds an OpenAI-compatible AI connection to the
| Stowline demo workspace, so the cost simulation can answer on another
| provider with --connection. The key is the install's open-weight key
| (ai.managed.openweight.api_key), so it never appears in a command
| line: set that to the provider's key for the one run that adds it.
| The connection is validated against the provider as it saves. Run for
| a name that is already there, it checks that connection again.
|
*/

package commands

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/ai"
	"github.com/dimmihq/dimmi/app/enums"
)

/*
AIDemoConnectionCommand adds the connection.
*/
type AIDemoConnectionCommand struct {
	App *app.App
}

/*
Name returns ai:demo-connection.
*/
func (AIDemoConnectionCommand) Name() string { return "ai:demo-connection" }

/*
Description says what the command does.
*/
func (AIDemoConnectionCommand) Description() string {
	return "Add an OpenAI-compatible AI connection to the Stowline demo workspace, with the install's open-weight key"
}

/*
Flags asks for the connection's name and address.
*/
func (AIDemoConnectionCommand) Flags(flags *console.Flags) {
	flags.String("name", "", "the connection's name, as --connection of ai:simulate takes it")
	flags.String("base-url", "", "the provider's OpenAI-compatible base URL")
	flags.Bool("prompt-cache", "the provider caches prompts, so the tool layer applies")
}

/*
Handle validates and saves the connection.
*/
func (c AIDemoConnectionCommand) Handle(
	ctx context.Context, _ *foundation.Application, in *console.Input, out *console.Output,
) error {
	name, base := in.String("name"), in.String("base-url")
	if name == "" || base == "" {
		return errors.New("--name and --base-url are both required")
	}
	key := c.App.Config.AI.Managed.OpenWeight.APIKey
	if key == "" {
		return errors.New("set ai.managed.openweight.api_key to the provider's key for this run")
	}
	owner, err := demoOwner(ctx, c.App)
	if err != nil {
		return err
	}
	wctx := tenancy.With(ctx, owner.Workspace.ID.String())
	if id, err := connectionNamed(ctx, c.App, owner.Workspace.ID, name); err == nil && id != nil {
		tested, err := c.App.Actions.AI.TestProviderKey.Handle(wctx, ai.TestKeyInput{Membership: owner, KeyID: *id})
		if err != nil {
			return fmt.Errorf("check connection: %w", err)
		}
		out.Info("%s is already there; checked again: %s (%s).", tested.Name, tested.Status,
			reasonOf(tested.StatusReason))
		return nil
	}
	config := map[string]any{}
	if in.Bool("prompt-cache") {
		config["prompt_cache"] = true
	}
	saved, err := c.App.Actions.AI.CreateProviderKey.Handle(wctx, ai.CreateKeyInput{
		Membership: owner, Name: name, Provider: enums.AIProviderOpenAICompatible, BaseURL: base, APIKey: key,
		Config: config,
	})
	if err != nil {
		return fmt.Errorf("add connection: %w", err)
	}
	out.Info("Added %s (%s) to the Stowline workspace: %s (%s).", saved.Name, base, saved.Status,
		reasonOf(saved.StatusReason))
	return nil
}

/*
reasonOf reads a connection's status reason, empty when it has none.
*/
func reasonOf(reason *string) string {
	if reason == nil {
		return ""
	}
	return *reason
}
