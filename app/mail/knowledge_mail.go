/*
|--------------------------------------------------------------------------
| Knowledge Source Mail
|--------------------------------------------------------------------------
|
| When a knowledge source that worked stops syncing, the agent answers
| from a copy that no longer moves, so the workspace's owners and admins
| hear once, with the error and the link to fix the source.
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
KnowledgeSourceFailingMail tells an owner or admin that Source of
Workspace stopped syncing with Error, and links its settings.
*/
type KnowledgeSourceFailingMail struct {
	Name        string `json:"name"`
	Workspace   string `json:"workspace"`
	Source      string `json:"source"`
	Error       string `json:"error"`
	SettingsURL string `json:"settings_url"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (KnowledgeSourceFailingMail) Envelope() mail.Envelope {
	return mail.Envelope{}
}

/*
Content renders the mail, its subject naming the source.
*/
func (m KnowledgeSourceFailingMail) Content(ctx context.Context) (mail.Content, error) {
	c := mail.View(accounts.KnowledgeSourceFailing(m.Name, m.Workspace, m.Source, m.Error, m.SettingsURL))
	c.Subject = i18n.T(ctx, "mail.knowledge_source_failing.subject", i18n.Args{"source": m.Source})

	return c, nil
}

/*
RegisterKnowledgeNotices lets the mail be queued that tells admins a
knowledge source stopped syncing.
*/
func RegisterKnowledgeNotices(m *mail.Manager) {
	mail.Register[KnowledgeSourceFailingMail](m, "knowledge_source_failing")
}
