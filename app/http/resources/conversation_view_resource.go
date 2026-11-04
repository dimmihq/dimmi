/*
|--------------------------------------------------------------------------
| Conversation View Resource
|--------------------------------------------------------------------------
|
| A saved view as the API shows it: its name, icon, scope, conditions,
| order and defaults, and the warnings about conditions naming rows that
| are gone.
|
*/

package resources

import (
	"encoding/json"
	"time"

	conversationviews "github.com/dimmihq/dimmi/app/actions/conversation_views"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ConversationView is a saved view. OwnerID is the member who keeps a
personal view, nil for a shared one.
*/
type ConversationView struct {
	ID           string          `json:"id"`
	Object       string          `json:"object"`
	Name         string          `json:"name"`
	Icon         *string         `json:"icon"`
	Scope        string          `json:"scope"`
	OwnerID      *string         `json:"owner_id"`
	Definition   json.RawMessage `json:"definition"`
	Sort         *string         `json:"sort"`
	StatusFilter *string         `json:"status_filter"`
	Position     int32           `json:"position"`
	Warnings     []ViewWarning   `json:"warnings"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

/*
ViewWarning flags one condition of a view.
*/
type ViewWarning struct {
	Path string `json:"path"`
	Code string `json:"code"`
}

/*
NewConversationView shapes the view.
*/
func NewConversationView(v conversationviews.View) ConversationView {
	row := v.View
	out := ConversationView{
		ID: models.PublicID(models.PrefixConversationView, row.ID), Object: "conversation_view", Name: row.Name,
		Icon: row.Icon, Scope: row.Scope, Definition: row.Definition, Sort: row.Sort, StatusFilter: row.StatusFilter,
		Position: row.Position, Warnings: []ViewWarning{}, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}

	if row.OwnerMemberID != nil {
		out.OwnerID = new(models.PublicID(models.PrefixMember, *row.OwnerMemberID))
	}

	seen := map[string]bool{}

	for _, w := range v.Warnings {
		if !seen[w.Path] {
			seen[w.Path] = true
			out.Warnings = append(out.Warnings, ViewWarning(w))
		}
	}

	return out
}

/*
NewConversationViews shapes the views.
*/
func NewConversationViews(views []conversationviews.View) []ConversationView {
	out := make([]ConversationView, len(views))
	for i, v := range views {
		out[i] = NewConversationView(v)
	}

	return out
}
