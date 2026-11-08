/*
|--------------------------------------------------------------------------
| Procedure Resources
|--------------------------------------------------------------------------
|
| How a procedure reads over the API: its working copy, whether that
| differs from what dimmi follows, how often dimmi followed it in the
| last 30 days, and the lock version a save names.
|
*/

package resources

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Procedure is a procedure with its working copy.
*/
type Procedure struct {
	ID                    string          `json:"id"`
	Object                string          `json:"object"`
	Name                  string          `json:"name"`
	Status                string          `json:"status"`
	LockVersion           int32           `json:"lock_version"`
	Definition            json.RawMessage `json:"definition"`
	HasUnpublishedChanges bool            `json:"has_unpublished_changes"`
	Uses                  int32           `json:"uses"`
	PublishedAt           *time.Time      `json:"published_at"`
	CreatedAt             time.Time       `json:"created_at"`
	UpdatedAt             time.Time       `json:"updated_at"`
}

/*
NewProcedure shapes one procedure, with how often dimmi followed it.
*/
func NewProcedure(p models.Procedure, uses int32) Procedure {
	return Procedure{
		ID: models.PublicID(models.PrefixProcedure, p.ID), Object: "procedure", Name: p.Name, Status: p.Status,
		LockVersion: p.LockVersion, Definition: p.Draft, Uses: uses, PublishedAt: p.PublishedAt,
		HasUnpublishedChanges: p.Status == "live" && !sameJSON(p.Draft, p.Live),
		CreatedAt:             p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

/*
sameJSON reports whether two stored JSON values are the same document.
*/
func sameJSON(a, b json.RawMessage) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return bytes.Equal(a, b)
	}

	ca, _ := json.Marshal(x)
	cb, _ := json.Marshal(y)

	return bytes.Equal(ca, cb)
}
