/*
|--------------------------------------------------------------------------
| Update Onboarding
|--------------------------------------------------------------------------
|
| A new workspace opens on its setup steps, and its owners and admins
| walk them before the app opens. The step they reached is kept on the
| workspace, so they resume on any device, and finishing opens the app
| for everyone. Finishing is for good: a workspace that finished never
| goes back to setup, whatever step is sent later.
|
*/

package workspaces

import (
	"context"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
OnboardingSteps are the setup steps of a new workspace, in order.
*/
func OnboardingSteps() []string {
	return []string{"workspace", "logo", "channels", "tags", "team", "knowledge", "test_conversations", "done"}
}

/*
UpdateOnboardingInput is who moves the workspace's setup along, the step
they reached and whether they finished.
*/
type UpdateOnboardingInput struct {
	Membership authz.Membership
	Step       string
	Complete   bool
}

/*
UpdateOnboarding records how far a workspace's setup has come.

Rules:
  - The caller needs settings:write.
  - The step is one of OnboardingSteps.
  - Finishing keeps the first time the workspace finished.

Errors: [exceptions.ErrForbidden], [exceptions.ErrOnboardingStepUnknown].
*/
type UpdateOnboarding struct {
	DB    *database.DB
	Clock clock.Clock
}

/*
Handle stores the step and, when finishing, the time.
*/
func (a *UpdateOnboarding) Handle(ctx context.Context, in UpdateOnboardingInput) (models.Workspace, error) {
	if !in.Membership.Can("settings:write") {
		return models.Workspace{}, exceptions.ErrForbidden
	}

	if !slices.Contains(OnboardingSteps(), in.Step) {
		return models.Workspace{}, exceptions.ErrOnboardingStepUnknown
	}

	var updated models.Workspace

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		updated, err = models.New(tx).UpdateWorkspaceOnboarding(ctx, models.UpdateWorkspaceOnboardingParams{
			ID: in.Membership.Workspace.ID, Step: in.Step, Complete: in.Complete, Now: a.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("update workspace onboarding: %w", err)
		}

		return nil
	})

	return updated, err
}
