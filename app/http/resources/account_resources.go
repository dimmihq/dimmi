/*
|--------------------------------------------------------------------------
| Account Resources
|--------------------------------------------------------------------------
|
| Answers about changes to a person's own account. A pending move to a new
| address shows it masked, since the mail it went to has not proved it
| yet; a confirmed one shows it whole to whoever holds the link, who just
| proved they read that mailbox. A password change says how many access
| tokens still work, because they are not ended by it.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/members"
	"github.com/dimmihq/dimmi/app/actions/users"
	"github.com/dimmihq/dimmi/app/models"
)

/*
EmailChange is a move to a new address: pending until the link is
opened, then confirmed.
*/
type EmailChange struct {
	Object      string     `json:"object"`
	ID          string     `json:"id"`
	Email       string     `json:"email"`
	Status      string     `json:"status"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	ConfirmedAt *time.Time `json:"confirmed_at,omitempty"`
	UserID      string     `json:"user_id"`
}

/*
NewEmailChange shapes a move to a new address.
*/
func NewEmailChange(c users.EmailChange) EmailChange {
	status := "pending"
	if c.ConfirmedAt != nil {
		status = "confirmed"
	}

	return EmailChange{
		Object: "email_change", ID: models.PublicID(models.PrefixOneTimeToken, c.ID), Email: c.Email, Status: status,
		ExpiresAt: utc(c.ExpiresAt), ConfirmedAt: utc(c.ConfirmedAt), UserID: models.PublicID(models.PrefixUser, c.UserID),
	}
}

/*
PasswordChange is the outcome of a new password.
*/
type PasswordChange struct {
	Object          string `json:"object"`
	TokensRemaining int64  `json:"tokens_remaining"`
}

/*
DeletionWorkspace is a workspace named in a deletion preview.
*/
type DeletionWorkspace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

/*
DeletionPreview says what deleting the account would do.
*/
type DeletionPreview struct {
	Object        string              `json:"object"`
	Blocking      []DeletionWorkspace `json:"blocking"`
	WillBeDeleted []DeletionWorkspace `json:"will_be_deleted"`
}

/*
NewDeletionPreview shapes a deletion preview.
*/
func NewDeletionPreview(p members.DeletionPreview) DeletionPreview {
	return DeletionPreview{
		Object:        "user_deletion_preview",
		Blocking:      deletionWorkspaces(p.Blocking),
		WillBeDeleted: deletionWorkspaces(p.WillBeDeleted),
	}
}

func deletionWorkspaces(in []members.DeletedWorkspace) []DeletionWorkspace {
	out := make([]DeletionWorkspace, 0, len(in))
	for _, w := range in {
		out = append(out, DeletionWorkspace{
			ID: models.PublicID(models.PrefixWorkspace, w.ID), Name: w.Name, Slug: w.Slug,
		})
	}

	return out
}
