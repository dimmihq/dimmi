/*
|--------------------------------------------------------------------------
| Saved Reply Resources
|--------------------------------------------------------------------------
|
| A saved reply as the API shows it: its text, whether it is shared or
| the caller's own, and the macro actions it applies, every id public.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/savedreplies"
	"github.com/dimmihq/dimmi/app/models"
)

/*
SavedReplyActions is what a reply does to the conversation it is sent
on. Empty fields are left out.
*/
type SavedReplyActions struct {
	Status       *string  `json:"status,omitempty"`
	SnoozePreset *string  `json:"snooze_preset,omitempty"`
	AssigneeID   *string  `json:"assignee_id,omitempty"`
	TeamID       *string  `json:"team_id,omitempty"`
	TagIDs       []string `json:"tag_ids"`
	Priority     *string  `json:"priority,omitempty"`
}

/*
SavedReply is one reply.
*/
type SavedReply struct {
	ID        string            `json:"id"`
	Object    string            `json:"object"`
	Title     string            `json:"title"`
	Body      string            `json:"body"`
	Shortcut  *string           `json:"shortcut"`
	Shared    bool              `json:"shared"`
	OwnerID   *string           `json:"owner_id"`
	Actions   SavedReplyActions `json:"actions"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

/*
NewSavedReply shapes the reply.
*/
func NewSavedReply(r models.SavedReply) SavedReply {
	out := SavedReply{
		ID: models.PublicID(models.PrefixSavedReply, r.ID), Object: "saved_reply", Title: r.Title, Body: r.Body,
		Shortcut: r.Shortcut, Shared: r.OwnerMemberID == nil, Actions: newSavedReplyActions(r),
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}

	if r.OwnerMemberID != nil {
		owner := models.PublicID(models.PrefixMember, *r.OwnerMemberID)
		out.OwnerID = &owner
	}

	return out
}

/*
NewSavedReplies shapes a list of replies, an empty list when there are
none.
*/
func NewSavedReplies(rows []models.SavedReply) []SavedReply {
	out := make([]SavedReply, 0, len(rows))
	for _, r := range rows {
		out = append(out, NewSavedReply(r))
	}

	return out
}

/*
newSavedReplyActions shapes a reply's stored macro.
*/
func newSavedReplyActions(r models.SavedReply) SavedReplyActions {
	m := savedreplies.ReadMacro(r.Actions)
	out := SavedReplyActions{
		Status: m.Status, SnoozePreset: m.SnoozePreset, Priority: m.Priority, TagIDs: make([]string, 0, len(m.TagIDs)),
	}

	if m.AssigneeID != nil {
		id := models.PublicID(models.PrefixMember, *m.AssigneeID)
		out.AssigneeID = &id
	}

	if m.TeamID != nil {
		id := models.PublicID(models.PrefixTeam, *m.TeamID)
		out.TeamID = &id
	}

	for _, tag := range m.TagIDs {
		out.TagIDs = append(out.TagIDs, models.PublicID(models.PrefixTag, tag))
	}

	return out
}
