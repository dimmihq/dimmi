/*
|--------------------------------------------------------------------------
| Onboarding Controller
|--------------------------------------------------------------------------
|
| PATCH /workspace/onboarding records how far a new workspace's setup has
| come, and finishing opens the app. POST
| /workspace/onboarding/test-conversations starts the five made-up
| conversations a team can practice on.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/workspaces"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
OnboardingController serves a workspace's setup steps.
*/
type OnboardingController struct {
	Save              *workspaces.UpdateOnboarding
	TestConversations *conversations.CreateTestConversations
}

/*
TestConversationsOutput is how many practice conversations started.
*/
type TestConversationsOutput struct {
	Body fhttp.Resource[resources.TestConversations]
}

/*
Update stores the step reached and answers with the workspace.
*/
func (c *OnboardingController) Update(
	ctx context.Context, in *requests.UpdateOnboardingRequest,
) (*WorkspaceOutput, error) {
	updated, err := c.Save.Handle(ctx, workspaces.UpdateOnboardingInput{
		Membership: membership(ctx), Step: in.Body.Step, Complete: in.Body.Completed,
	})
	if err != nil {
		return nil, err
	}

	return &WorkspaceOutput{Body: fhttp.NewResource(resources.NewWorkspace(updated))}, nil
}

/*
StoreTestConversations starts the practice conversations.
*/
func (c *OnboardingController) StoreTestConversations(
	ctx context.Context, _ *requests.NoInput,
) (*TestConversationsOutput, error) {
	created, err := c.TestConversations.Handle(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}

	return &TestConversationsOutput{Body: fhttp.NewResource(resources.NewTestConversations(created))}, nil
}
