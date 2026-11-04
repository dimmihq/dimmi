/*
|--------------------------------------------------------------------------
| Contact Identity And Duplicate Resources
|--------------------------------------------------------------------------
|
| How a customer appears on a channel, and who else looks like the same
| person, with the reasons a teammate weighs before merging.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ContactIdentity is one way a customer appears on a channel.
*/
type ContactIdentity struct {
	ID        string    `json:"id"`
	Object    string    `json:"object"`
	Type      string    `json:"type"`
	Value     string    `json:"value"`
	Verified  bool      `json:"verified"`
	CreatedAt time.Time `json:"created_at"`
}

/*
NewContactIdentities shapes the identities, in the order given.
*/
func NewContactIdentities(rows []models.ContactIdentity) []ContactIdentity {
	out := make([]ContactIdentity, 0, len(rows))
	for _, i := range rows {
		out = append(out, ContactIdentity{
			ID: models.PublicID(models.PrefixContactIdentity, i.ID), Object: "contact_identity", Type: i.Type,
			Value: i.Value, Verified: i.Verified, CreatedAt: i.CreatedAt,
		})
	}

	return out
}

/*
ContactDuplicate is a contact that looks like the same person, and why.
*/
type ContactDuplicate struct {
	Object  string         `json:"object"`
	Contact ContactSummary `json:"contact"`
	Reasons []string       `json:"reasons"`
}

/*
NewContactDuplicates shapes the possible duplicates, in the order given.
*/
func NewContactDuplicates(rows []contacts.Duplicate) []ContactDuplicate {
	out := make([]ContactDuplicate, 0, len(rows))
	for _, d := range rows {
		out = append(out, ContactDuplicate{
			Object: "contact_duplicate", Contact: NewContactSummary(d.Contact), Reasons: d.Reasons,
		})
	}

	return out
}
