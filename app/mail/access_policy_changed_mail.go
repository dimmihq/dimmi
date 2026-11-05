/*
|--------------------------------------------------------------------------
| Access Policy Changed Mail
|--------------------------------------------------------------------------
|
| A change to who may sign in to a workspace is told to every owner but
| the one who made it, so it never goes unnoticed. The mail names no
| values: an office's addresses have no business sitting in a mailbox.
|
*/

package mail

import (
	"context"

	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/mail"

	"github.com/dimmihq/dimmi/resources/views/mail/accounts"
)

/*
AccessPolicyChangedMail tells an owner the Workspace's sign-in rules
changed, and links the page that shows them.
*/
type AccessPolicyChangedMail struct {
	Name        string `json:"name"`
	Workspace   string `json:"workspace"`
	SettingsURL string `json:"settings_url"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (AccessPolicyChangedMail) Envelope() mail.Envelope {
	return mail.Envelope{}
}

/*
Content renders the mail, its subject naming the workspace.
*/
func (m AccessPolicyChangedMail) Content(ctx context.Context) (mail.Content, error) {
	c := mail.View(accounts.AccessPolicyChanged(m.Name, m.Workspace, m.SettingsURL))
	c.Subject = i18n.T(ctx, "mail.access_policy_changed.subject", i18n.Args{"workspace": m.Workspace})

	return c, nil
}
