/*
|--------------------------------------------------------------------------
| Saved Reply Requests
|--------------------------------------------------------------------------
|
| Inputs of the endpoints on saved replies and macros. A macro names the
| teammate, team and tags by public id; the action checks they belong to
| the workspace.
|
*/

package requests

/*
SavedReplyMacroInput is the macro a reply carries: a status to move the
conversation to, with a snooze preset when snoozing, a teammate or team
to assign it to, tags to add and a priority to set.
*/
type SavedReplyMacroInput struct {
	Status       *string  `json:"status,omitempty"        validate:"omitempty,oneof=open closed snoozed"`
	SnoozePreset *string  `json:"snooze_preset,omitempty" validate:"omitempty,oneof=later_today tomorrow next_week one_week"` //nolint:lll // a struct tag cannot wrap
	AssigneeID   *string  `json:"assignee_id,omitempty"   validate:"omitempty,max=64"`
	TeamID       *string  `json:"team_id,omitempty"       validate:"omitempty,max=64"`
	TagIDs       []string `json:"tag_ids,omitempty"       validate:"omitempty,max=10,dive,max=64"`
	Priority     *string  `json:"priority,omitempty"      validate:"omitempty,oneof=none low medium high urgent"`
}

/*
SavedReplyRequest is the input of the endpoints that name only a reply.
*/
type SavedReplyRequest struct {
	Reply string `path:"saved_reply"`
}

/*
StoreSavedReplyRequest is the input of POST /saved-replies.
*/
type StoreSavedReplyRequest struct {
	Body struct {
		Title    string                `json:"title"              validate:"required,min=1,max=120"`
		Body     string                `json:"body"               validate:"required,min=1,max=65536"`
		Shortcut *string               `json:"shortcut,omitempty" validate:"omitempty,max=40"`
		Shared   *bool                 `json:"shared,omitempty"`
		Actions  *SavedReplyMacroInput `json:"actions,omitempty"`
	}
}

/*
UpdateSavedReplyRequest is the input of PATCH /saved-replies/{saved_reply}.
A null shortcut removes it.
*/
type UpdateSavedReplyRequest struct {
	Reply string `path:"saved_reply"`
	Body  struct {
		Title    *string               `json:"title,omitempty"    validate:"omitempty,min=1,max=120"`
		Body     *string               `json:"body,omitempty"     validate:"omitempty,min=1,max=65536"`
		Shortcut *string               `json:"shortcut,omitempty" validate:"omitempty,max=40"`
		Shared   *bool                 `json:"shared,omitempty"`
		Actions  *SavedReplyMacroInput `json:"actions,omitempty"`
	}
	RawBody []byte
}
