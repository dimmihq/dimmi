/*
|--------------------------------------------------------------------------
| Changelog Mail
|--------------------------------------------------------------------------
|
| What a help center's changelog mails to the people who asked for its
| updates: first the link that confirms an address really wants them,
| then each entry the team chose to send. Every entry mail says how to
| stop, in its footer and in the List-Unsubscribe headers mail apps turn
| into a one-click button.
|
*/

package mail

import (
	"context"

	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/mail"

	"github.com/dimmihq/dimmi/resources/views/mail/changelog"
)

/*
ChangelogConfirmMail asks the owner of an address to confirm they want
the HelpCenter's updates, through Link.
*/
type ChangelogConfirmMail struct {
	HelpCenter string `json:"help_center"`
	Brand      string `json:"brand"`
	Link       string `json:"link"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (ChangelogConfirmMail) Envelope() mail.Envelope {
	return mail.Envelope{Tags: map[string]string{"kind": "changelog_confirm"}}
}

/*
Content renders the mail.
*/
func (m ChangelogConfirmMail) Content(ctx context.Context) (mail.Content, error) {
	c := mail.View(changelog.Confirm(m.HelpCenter, m.Brand, m.Link))
	c.Subject = i18n.T(ctx, "mail.changelog_confirm.subject", i18n.Args{"help_center": m.HelpCenter})
	return c, nil
}

/*
ChangelogEntryMail is a new entry for one subscriber. Unsubscribe is the
page that stops these; OneClick takes the mail app's one-click POST.
*/
type ChangelogEntryMail struct {
	Entry    changelog.Entry `json:"entry"`
	OneClick string          `json:"one_click"`
}

/*
Envelope names the entry and how to unsubscribe in one click.
*/
func (m ChangelogEntryMail) Envelope() mail.Envelope {
	return mail.Envelope{
		Subject: m.Entry.Title,
		Headers: map[string]string{
			"List-Unsubscribe":      "<" + m.OneClick + ">",
			"List-Unsubscribe-Post": "List-Unsubscribe=One-Click",
		},
		Tags: map[string]string{"kind": "changelog_entry"},
	}
}

/*
Content renders the mail.
*/
func (m ChangelogEntryMail) Content(context.Context) (mail.Content, error) {
	c := mail.View(changelog.NewEntry(m.Entry))
	c.Subject = m.Entry.Title
	return c, nil
}
