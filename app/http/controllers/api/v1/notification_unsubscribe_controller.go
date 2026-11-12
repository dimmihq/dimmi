/*
|--------------------------------------------------------------------------
| Notification Unsubscribe Controller
|--------------------------------------------------------------------------
|
| The page an unsubscribe link in a teammate email opens, with no
| sign-in: GET reads what the link names, POST turns that email off and
| DELETE turns it back on. The link is the proof and names only its own
| member and kind of email.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/notifications"
	"github.com/dimmihq/dimmi/app/http/requests"
)

/*
NotificationUnsubscribeController serves the unsubscribe page.
*/
type NotificationUnsubscribeController struct {
	Unsubscribes *notifications.Unsubscribes
}

/*
NotificationUnsubscribe is what the page shows.
*/
type NotificationUnsubscribe struct {
	Object      string `json:"object"`
	Workspace   string `json:"workspace"`
	Email       string `json:"email"`
	Trigger     string `json:"trigger"`
	Label       string `json:"label"`
	Subscribed  bool   `json:"subscribed"`
	SettingsURL string `json:"settings_url"`
}

/*
NotificationUnsubscribeOutput answers the page.
*/
type NotificationUnsubscribeOutput struct {
	Body fhttp.Resource[NotificationUnsubscribe]
}

/*
Show reads what the link names.
*/
func (c *NotificationUnsubscribeController) Show(ctx context.Context, in *requests.NotificationUnsubscribeRequest,
) (*NotificationUnsubscribeOutput, error) {
	return unsubscribeOutput(c.Unsubscribes.Show(ctx, in.Token))
}

/*
Update turns the link's email off.
*/
func (c *NotificationUnsubscribeController) Update(ctx context.Context, in *requests.NotificationUnsubscribeRequest,
) (*NotificationUnsubscribeOutput, error) {
	return unsubscribeOutput(c.Unsubscribes.Unsubscribe(ctx, in.Token))
}

/*
Destroy turns the link's email back on.
*/
func (c *NotificationUnsubscribeController) Destroy(ctx context.Context, in *requests.NotificationUnsubscribeRequest,
) (*NotificationUnsubscribeOutput, error) {
	return unsubscribeOutput(c.Unsubscribes.Resubscribe(ctx, in.Token))
}

func unsubscribeOutput(s notifications.UnsubscribeState, err error) (*NotificationUnsubscribeOutput, error) {
	if err != nil {
		return nil, err
	}
	return &NotificationUnsubscribeOutput{Body: fhttp.NewResource(NotificationUnsubscribe{
		Object: "notification_unsubscribe", Workspace: s.Workspace, Email: s.Email, Trigger: s.Trigger,
		Label: s.Label, Subscribed: s.Subscribed, SettingsURL: s.SettingsURL,
	})}, nil
}
