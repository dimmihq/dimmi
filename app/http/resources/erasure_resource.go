/*
|--------------------------------------------------------------------------
| Contact Erasure Resource
|--------------------------------------------------------------------------
|
| A request to erase a customer as the API shows it: where it stands,
| when the wipe is due, and what it removed. The identities it holds
| sealed are never shown. A canceled request lists the addresses it
| could not give back.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/app/models"
)

/*
ContactErasure is one request.
*/
type ContactErasure struct {
	ID                   string          `json:"id"`
	Object               string          `json:"object"`
	ContactID            string          `json:"contact_id"`
	Status               string          `json:"status"`
	ScheduledFor         time.Time       `json:"scheduled_for"`
	CompletedAt          *time.Time      `json:"completed_at"`
	Counts               json.RawMessage `json:"counts"`
	IdentitiesConflicted []string        `json:"identities_conflicted,omitempty"`
	CreatedAt            time.Time       `json:"created_at"`
}

/*
NewContactErasure shapes the request.
*/
func NewContactErasure(e models.ContactErasure) ContactErasure {
	return ContactErasure{
		ID: models.PublicID(models.PrefixContactErasure, e.ID), Object: "contact_erasure",
		ContactID: models.PublicID(models.PrefixContact, e.ContactID), Status: e.Status,
		ScheduledFor: e.ScheduledFor, CompletedAt: e.CompletedAt, Counts: e.Counts, CreatedAt: e.CreatedAt,
	}
}

/*
NewContactErasures shapes a list of requests, in the order given.
*/
func NewContactErasures(rows []models.ContactErasure) []ContactErasure {
	out := make([]ContactErasure, 0, len(rows))
	for _, row := range rows {
		out = append(out, NewContactErasure(row))
	}

	return out
}

/*
ContactExport is a customer's export and where it stands. URL is where
a ready export downloads from.
*/
type ContactExport struct {
	ID           string     `json:"id"`
	Object       string     `json:"object"`
	ContactID    string     `json:"contact_id"`
	Status       string     `json:"status"`
	IncludeNotes bool       `json:"include_notes"`
	SizeBytes    *int64     `json:"size_bytes"`
	ExpiresAt    *time.Time `json:"expires_at"`
	URL          *string    `json:"url"`
	CreatedAt    time.Time  `json:"created_at"`
}

/*
NewContactExport shapes the export.
*/
func NewContactExport(e models.ContactExport) ContactExport {
	out := ContactExport{
		ID: models.PublicID(models.PrefixContactExport, e.ID), Object: "contact_export",
		ContactID: models.PublicID(models.PrefixContact, e.ContactID), Status: e.Status, IncludeNotes: e.IncludeNotes,
		SizeBytes: e.SizeBytes, ExpiresAt: e.ExpiresAt, CreatedAt: e.CreatedAt,
	}

	if e.Status == "ready" {
		out.URL = new("/api/v1/contacts/" + out.ContactID + "/exports/" + out.ID + "/download")
	}

	return out
}
