/*
|--------------------------------------------------------------------------
| Report Digest Mail
|--------------------------------------------------------------------------
|
| A teammate who asked for it gets the reports' headline numbers each
| morning or each Monday: the period covered, each number with how it
| moved since the period before, and a link to the reports.
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
DigestLine is one number of a digest, written out.
*/
type DigestLine = accounts.DigestLine

/*
ReportDigestMail is a teammate's digest of Workspace for the days From
to To, daily or weekly.
*/
type ReportDigestMail struct {
	Name      string       `json:"name"`
	Workspace string       `json:"workspace"`
	Weekly    bool         `json:"weekly"`
	From      string       `json:"from"`
	To        string       `json:"to"`
	Lines     []DigestLine `json:"lines"`
	ReportURL string       `json:"report_url"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (ReportDigestMail) Envelope() mail.Envelope {
	return mail.Envelope{}
}

/*
Content renders the mail, its subject naming the workspace and period.
*/
func (m ReportDigestMail) Content(ctx context.Context) (mail.Content, error) {
	key := "mail.report_digest.daily_subject"
	if m.Weekly {
		key = "mail.report_digest.weekly_subject"
	}

	subject := i18n.T(ctx, key, i18n.Args{"workspace": m.Workspace})
	c := mail.View(accounts.ReportDigest(subject, m.Name, m.From, m.To, m.Lines, m.ReportURL))
	c.Subject = subject

	return c, nil
}

/*
RegisterReportDigests lets report digests be queued.
*/
func RegisterReportDigests(m *mail.Manager) {
	mail.Register[ReportDigestMail](m, "report_digest")
}
