/*
|--------------------------------------------------------------------------
| Attributes Required To Close
|--------------------------------------------------------------------------
|
| A workspace may require some conversation attributes before a
| teammate closes a conversation, such as why the customer wrote. A
| teammate's close, alone or in bulk, is refused while one is empty,
| naming each one missing. The AI, workflows, the customer and imports
| still close conversations as before.
|
*/

package conversations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"

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

	var missing []map[string]string

	for _, a := range required {
		if fieldvalues.Empty(values[a.Key]) {
			missing = append(missing, map[string]string{"key": a.Key, "label": a.Label})
		}
	}

	if len(missing) > 0 {
		return ErrAttributesRequired.With("missing", missing)
	}

	return nil
}

func wrapRequired(err error) error {
	if err != nil {
		return fmt.Errorf("check attributes required to close: %w", err)
	}

	return nil
}
