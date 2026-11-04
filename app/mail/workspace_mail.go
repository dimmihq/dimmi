/*
|--------------------------------------------------------------------------
| Workspace Mail
|--------------------------------------------------------------------------
|
| Owners hear about every step of a workspace's way to deletion: when it
| goes to the trash, a few days before it is purged for good, and when
| someone brings it back. A member made owner because the workspace was
| left without one is told why. Each mail names the workspace in its
| subject, so an owner of several knows at once which one it is about.
|
*/

package mail

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/view"

	"github.com/dimmihq/dimmi/resources/views/mail/accounts"
)

/*
WorkspaceDeletionScheduledMail tells an owner that DeletedBy moved the
workspace to the trash, restorable until PurgeAfter.
*/
type WorkspaceDeletionScheduledMail struct {
	Name       string    `json:"name"`
	Workspace  string    `json:"workspace"`
	DeletedBy  string    `json:"deleted_by"`
	PurgeAfter time.Time `json:"purge_after"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (WorkspaceDeletionScheduledMail) Envelope() mail.Envelope {
	return mail.Envelope{}
}

/*
Content renders the mail.
*/
func (m WorkspaceDeletionScheduledMail) Content(ctx context.Context) (mail.Content, error) {
	body := accounts.WorkspaceDeletionScheduled(m.Name, m.Workspace, m.DeletedBy, day(m.PurgeAfter))

	return contentAbout(ctx, "mail.workspace_deletion_scheduled.subject", about(m.Workspace), body), nil
}

/*
WorkspaceDeletionReminderMail tells an owner the workspace in the trash
is purged at PurgeAfter.
*/
type WorkspaceDeletionReminderMail struct {
	Name       string    `json:"name"`
	Workspace  string    `json:"workspace"`
	PurgeAfter time.Time `json:"purge_after"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (WorkspaceDeletionReminderMail) Envelope() mail.Envelope {
	return mail.Envelope{}
}

/*
Content renders the mail.
*/
func (m WorkspaceDeletionReminderMail) Content(ctx context.Context) (mail.Content, error) {
	body := accounts.WorkspaceDeletionReminder(m.Name, m.Workspace, day(m.PurgeAfter))

	return contentAbout(ctx, "mail.workspace_deletion_reminder.subject", about(m.Workspace), body), nil
}

/*
WorkspaceRestoredMail tells an owner that RestoredBy took the workspace
out of the trash, linking the app at AppURL.
*/
type WorkspaceRestoredMail struct {
	Name       string `json:"name"`
	Workspace  string `json:"workspace"`
	RestoredBy string `json:"restored_by"`
	AppURL     string `json:"app_url"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (WorkspaceRestoredMail) Envelope() mail.Envelope {
	return mail.Envelope{}
}

/*
Content renders the mail.
*/
func (m WorkspaceRestoredMail) Content(ctx context.Context) (mail.Content, error) {
	body := accounts.WorkspaceRestored(m.Name, m.Workspace, m.RestoredBy, m.AppURL)

	return contentAbout(ctx, "mail.workspace_restored.subject", about(m.Workspace), body), nil
}

/*
OwnerPromotedBySystemMail tells a member they were made an owner of the
workspace, which had none left, linking its members at MembersURL.
*/
type OwnerPromotedBySystemMail struct {
	Name       string `json:"name"`
	Workspace  string `json:"workspace"`
	MembersURL string `json:"members_url"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (OwnerPromotedBySystemMail) Envelope() mail.Envelope {
	return mail.Envelope{}
}

/*
Content renders the mail.
*/
func (m OwnerPromotedBySystemMail) Content(ctx context.Context) (mail.Content, error) {
	body := accounts.OwnerPromotedBySystem(m.Name, m.Workspace, m.MembersURL)

	return contentAbout(ctx, "mail.owner_promoted_by_system.subject", about(m.Workspace), body), nil
}

/*
contentAbout renders the component with the subject translated in the
mail's language, filled with the arguments.
*/
func contentAbout(ctx context.Context, subject string, args i18n.Args, component view.Component) mail.Content {
	c := mail.View(component)
	c.Subject = i18n.T(ctx, subject, args)

	return c
}

/*
about returns the arguments of a subject that names the workspace.
*/
func about(workspace string) i18n.Args {
	return i18n.Args{"workspace": workspace}
}

/*
day writes the date of the moment, in UTC, the way a person reads a
deadline.
*/
func day(t time.Time) string {
	return t.UTC().Format("January 2, 2006")
}
