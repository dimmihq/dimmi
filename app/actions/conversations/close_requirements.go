/*
|--------------------------------------------------------------------------
| Attributes Required To Close
|--------------------------------------------------------------------------
|
| A workspace may require some conversation attributes before a
| teammate closes a conversation, such as why the customer wrote. A
| teammate's close, alone or in bulk, is refused while one is empty,
| naming each one missing; one shown only to teams the teammate is not
| in does not hold them back. The AI, workflows, the customer and
| imports still close conversations as before.
|
| An attribute shown only to some teams is seen, read and written only
| by their members, in the inbox and through the API alike.
|
*/

package conversations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/fieldvalues"
)

/*
ErrAttributesRequired refuses a close while required conversation
attributes are empty; its missing list names each by key and label.
*/
var ErrAttributesRequired = fault.New(fault.Invalid, "conversation_attributes_required",
	"Fill the required conversation attributes before closing.").OnField("status")

/*
checkRequiredToClose refuses a teammate's close while a required
attribute is empty.
*/
func (w *Writer) checkRequiredToClose(ctx context.Context, c models.Conversation) error {
	if w.Imported || w.Actor.Type != string(enums.AuthorTypeMember) {
		return nil
	}

	required, err := w.Q.ListRequiredToCloseConversationAttributes(ctx, c.WorkspaceID)
	if err != nil || len(required) == 0 {
		return wrapRequired(err)
	}

	row, err := w.Q.FindConversationAIValues(ctx, models.FindConversationAIValuesParams{
		WorkspaceID: c.WorkspaceID, ConversationID: c.ID,
	})
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return wrapRequired(err)
	}

	values := map[string]any{}
	_ = json.Unmarshal(row.Values, &values)

	teams, err := MemberTeamIDs(ctx, w.Q, c.WorkspaceID, w.Actor.ID)
	if err != nil {
		return err
	}

	var (
		missing []map[string]string
		labels  []string
	)

	for _, a := range required {
		if SeesAttribute(a.TeamIds, teams) && fieldvalues.Empty(values[a.Key]) {
			missing = append(missing, map[string]string{"key": a.Key, "label": a.Label})
			labels = append(labels, a.Label)
		}
	}

	if len(missing) == 0 {
		return nil
	}

	return fault.New(fault.Invalid, ErrAttributesRequired.Code(), "Fill "+listed(labels)+" before closing.").
		OnField("status").With("missing", missing)
}

/*
listed joins names into a phrase: "Issue type", "Issue type and Plan",
"Issue type, Plan and Seats".
*/
func listed(names []string) string {
	if len(names) == 1 {
		return names[0]
	}

	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

func wrapRequired(err error) error {
	if err != nil {
		return fmt.Errorf("check attributes required to close: %w", err)
	}

	return nil
}

/*
MemberTeamIDs returns the live teams a member is in.
*/
func MemberTeamIDs(ctx context.Context, q *models.Queries, workspaceID, memberID ids.UUID) ([]ids.UUID, error) {
	rows, err := q.ListMemberTeams(ctx, models.ListMemberTeamsParams{
		WorkspaceID: workspaceID, MemberIds: []ids.UUID{memberID},
	})
	if err != nil {
		return nil, fmt.Errorf("list member teams: %w", err)
	}

	out := make([]ids.UUID, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Team.ID)
	}

	return out, nil
}

/*
SeesAttribute reports whether a member of the teams sees an attribute
shown to the attribute's teams, every team when it names none.
*/
func SeesAttribute(attributeTeams, memberTeams []ids.UUID) bool {
	if len(attributeTeams) == 0 {
		return true
	}

	for _, t := range attributeTeams {
		if slices.Contains(memberTeams, t) {
			return true
		}
	}

	return false
}
