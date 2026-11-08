/*
|--------------------------------------------------------------------------
| Audience Resources
|--------------------------------------------------------------------------
|
| How a saved audience reads over the API: its rule, its last counted
| size, and, read alone, the workflows and audiences that use it.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/app/actions/audiences"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Audience is a saved audience. UsedBy is listed only on a single read.
*/
type Audience struct {
	ID           string          `json:"id"`
	Object       string          `json:"object"`
	Name         string          `json:"name"`
	Description  *string         `json:"description"`
	Rules        json.RawMessage `json:"rules"`
	LastEstimate json.RawMessage `json:"last_estimate"`
	LockVersion  int32           `json:"lock_version"`
	UsedBy       []AudienceUser  `json:"used_by"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

/*
AudienceUser is something that names a saved audience: a workflow or
another saved audience, and whether a live version runs on it.
*/
type AudienceUser struct {
	Object string `json:"object"`
	ID     string `json:"id"`
	Name   string `json:"name"`
	Live   bool   `json:"live"`
}

/*
NewAudience shapes a saved audience with what uses it.
*/
func NewAudience(a models.Audience, users []audiences.User) Audience {
	out := Audience{
		ID: models.PublicID(models.PrefixAudience, a.ID), Object: "audience", Name: a.Name,
		Description: a.Description, Rules: a.Rules, LastEstimate: a.LastEstimate, LockVersion: a.LockVersion,
		CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}

	if users != nil {
		out.UsedBy = []AudienceUser{}
	}

	for _, u := range users {
		prefix := models.PrefixWorkflow
		if u.Kind == "audience" {
			prefix = models.PrefixAudience
		}

		out.UsedBy = append(out.UsedBy, AudienceUser{
			Object: u.Kind, ID: models.PublicID(prefix, u.ID), Name: u.Name, Live: u.Live,
		})
	}

	return out
}
