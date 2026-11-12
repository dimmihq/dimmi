/*
|--------------------------------------------------------------------------
| Saved Replies
|--------------------------------------------------------------------------
|
| A reply without an owner is shared with the whole workspace; one with
| an owner is that member's own and nobody else sees it. Anyone who works
| conversations keeps their own replies; shared ones take
| saved_replies:manage_shared, since everyone's composer offers them.
|
| A reply may carry macro actions. They are stored here and applied by
| the composer through the conversation's own update once the reply is
| sent, so a macro can never do what its sender could not do by hand.
|
*/

package savedreplies

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Limits on what a workspace keeps.
*/
const (
	MaxInWorkspace = 2000
	MaxListed      = 500
	MaxMacroTags   = 10
	maxTitle       = 120
)

/*
shortcutConstraint keeps an owner's shortcuts apart.
*/
const shortcutConstraint = "saved_replies_workspace_id_owner_member_id_shortcut_key"

/*
Macro is what a reply does to the conversation it is sent on: a status
to move it to, with the snooze preset when snoozing, a teammate or team
to assign it to, tags to add and a priority to set. Every field is
optional; an empty macro is a plain reply.
*/
type Macro struct {
	Status       *string    `json:"status,omitempty"`
	SnoozePreset *string    `json:"snooze_preset,omitempty"`
	AssigneeID   *ids.UUID  `json:"assignee_id,omitempty"`
	TeamID       *ids.UUID  `json:"team_id,omitempty"`
	TagIDs       []ids.UUID `json:"tag_ids,omitempty"`
	Priority     *string    `json:"priority,omitempty"`
}

/*
macroStatuses are the statuses a macro may set.
*/
func macroStatuses() []string {
	return []string{"open", "closed", "snoozed"}
}

/*
snoozePresets are the snoozes a macro may pick.
*/
func snoozePresets() []string {
	return []string{"later_today", "tomorrow", "next_week", "one_week"}
}

/*
macroPriorities are the priorities a macro may set.
*/
func macroPriorities() []string {
	return []string{"none", "low", "medium", "high", "urgent"}
}

/*
ReadMacro decodes a reply's stored actions; anything unreadable is a
plain reply.
*/
func ReadMacro(raw json.RawMessage) Macro {
	var m Macro
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &m)
	}

	return m
}

/*
Fields are a reply's values. A nil field keeps the current value on an
update; Shared nil keeps a reply where it is and makes a new one
personal. ClearShortcut removes the shortcut.
*/
type Fields struct {
	Title         *string
	Body          *string
	Shortcut      *string
	ClearShortcut bool
	Shared        *bool
	Macro         *Macro
}

/*
canUse reports whether the member may see and insert the reply: a
shared one or their own.
*/
func canUse(m authz.Membership, r models.SavedReply) bool {
	return r.OwnerMemberID == nil || *r.OwnerMemberID == m.Member.ID
}

/*
canChange reports whether the member may change or remove a reply that
is shared or not.
*/
func canChange(m authz.Membership, shared bool) bool {
	if !m.Can("conversations:write") {
		return false
	}

	return !shared || m.Can("saved_replies:manage_shared")
}

/*
find returns the reply the member may use, or exceptions.ErrNotFound.
*/
func find(ctx context.Context, q *models.Queries, m authz.Membership, id ids.UUID) (models.SavedReply, error) {
	row, err := q.FindSavedReply(ctx, models.FindSavedReplyParams{WorkspaceID: m.Workspace.ID, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return row, exceptions.ErrNotFound
	}

	if err != nil {
		return row, fmt.Errorf("find saved reply: %w", err)
	}

	if !canUse(m, row) {
		return row, exceptions.ErrNotFound
	}

	return row, nil
}

/*
cleanTitle trims a title and refuses an empty or overlong one.
*/
func cleanTitle(title string) (string, error) {
	title = strings.TrimSpace(title)
	if title == "" || len([]rune(title)) > maxTitle {
		return "", exceptions.ErrSavedReplyTitleRequired
	}

	return title, nil
}

/*
cleanBody refuses a body with nothing but space in it.
*/
func cleanBody(body string) (string, error) {
	if strings.TrimSpace(body) == "" {
		return "", exceptions.ErrSavedReplyBodyRequired
	}

	return body, nil
}

/*
cleanShortcut trims a shortcut and its leading # or /, and treats an
empty one as none.
*/
func cleanShortcut(shortcut *string) *string {
	if shortcut == nil {
		return nil
	}

	s := strings.TrimLeft(strings.TrimSpace(*shortcut), "#/")
	if s == "" {
		return nil
	}

	return &s
}

/*
encodeMacro checks a macro against the workspace and returns it as it
is stored.
*/
func encodeMacro(ctx context.Context, q *models.Queries, ws ids.UUID, m Macro) (json.RawMessage, error) {
	if err := checkChoices(m); err != nil {
		return nil, err
	}

	if m.AssigneeID != nil {
		_, err := q.FindWorkspaceMember(ctx, models.FindWorkspaceMemberParams{WorkspaceID: ws, ID: *m.AssigneeID})
		if err != nil {
			return nil, notFoundAsInvalid(err)
		}
	}

	if m.TeamID != nil {
		if _, err := q.FindTeam(ctx, models.FindTeamParams{WorkspaceID: ws, ID: *m.TeamID}); err != nil {
			return nil, notFoundAsInvalid(err)
		}
	}

	for _, tag := range m.TagIDs {
		if _, err := q.FindTag(ctx, models.FindTagParams{WorkspaceID: ws, ID: tag}); err != nil {
			return nil, notFoundAsInvalid(err)
		}
	}

	raw, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("encode macro: %w", err)
	}

	return raw, nil
}

/*
checkChoices refuses a status, snooze or priority the inbox does not
have, a snooze preset without snoozing, and too many tags.
*/
func checkChoices(m Macro) error {
	if m.Status != nil && !slices.Contains(macroStatuses(), *m.Status) {
		return exceptions.ErrInvalidMacro
	}

	snoozing := m.Status != nil && *m.Status == "snoozed"
	if snoozing != (m.SnoozePreset != nil) {
		return exceptions.ErrInvalidMacro
	}

	if m.SnoozePreset != nil && !slices.Contains(snoozePresets(), *m.SnoozePreset) {
		return exceptions.ErrInvalidMacro
	}

	if m.Priority != nil && !slices.Contains(macroPriorities(), *m.Priority) {
		return exceptions.ErrInvalidMacro
	}

	if len(m.TagIDs) > MaxMacroTags {
		return exceptions.ErrInvalidMacro
	}

	return nil
}

/*
notFoundAsInvalid turns a missing teammate, team or tag into an invalid
macro and wraps anything else.
*/
func notFoundAsInvalid(err error) error {
	if errors.Is(err, database.ErrNotFound) {
		return exceptions.ErrInvalidMacro
	}

	return fmt.Errorf("check macro: %w", err)
}
