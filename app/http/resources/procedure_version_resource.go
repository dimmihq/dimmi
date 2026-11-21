/*
|--------------------------------------------------------------------------
| Procedure Version Resources
|--------------------------------------------------------------------------
|
| A published version of a procedure as the API reads it: its number,
| name and definition as published, who published it and why, and how
| it did over the last 90 days.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/app/actions/procedures"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ProcedureVersion is one published version of a procedure.
*/
type ProcedureVersion struct {
	ID                  string          `json:"id"`
	Object              string          `json:"object"`
	ProcedureID         string          `json:"procedure_id"`
	Version             int32           `json:"version"`
	Name                string          `json:"name"`
	Definition          json.RawMessage `json:"definition"`
	ChangeNote          *string         `json:"change_note"`
	PublishedByMemberID *string         `json:"published_by_member_id"`
	PublishedAt         time.Time       `json:"published_at"`
	Conversations       int32           `json:"conversations"`
	HandedOff           int32           `json:"handed_off"`
}

/*
NewProcedureVersion shapes one version, with how it did.
*/
func NewProcedureVersion(v procedures.Version) ProcedureVersion {
	return ProcedureVersion{
		ID: models.PublicID(models.PrefixProcedureVer, v.ID), Object: "procedure_version",
		ProcedureID: models.PublicID(models.PrefixProcedure, v.ProcedureID), Version: v.Version,
		Name: v.Name, Definition: v.Definition, ChangeNote: v.ChangeNote,
		PublishedByMemberID: publicPtr(models.PrefixMember, v.PublishedByMemberID), PublishedAt: v.PublishedAt,
		Conversations: v.Conversations, HandedOff: v.HandedOff,
	}
}
