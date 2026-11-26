/*
|--------------------------------------------------------------------------
| Option Descriptions And Order
|--------------------------------------------------------------------------
|
| Each option of a list may say what it means, such as "Billing: charges,
| invoices and refunds", so the AI picks it for the right conversations.
| Descriptions are kept only for options the list still has. Attributes
| sit in the order the team puts them in, which the details panel
| follows.
|
*/

package aiattributes

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxOptionDescription is the longest description of one option.
*/
const maxOptionDescription = 300

/*
ErrOptionDescriptions refuses a description of an option the list does
not have, or one too long.
*/
var ErrOptionDescriptions = fault.New(fault.Invalid, "option_descriptions_invalid",
	"Describe only the list's own options, in 300 characters at most each.").OnField("option_descriptions")

/*
OptionDescriptions reads what each option of an attribute means, by
option.
*/
func OptionDescriptions(a models.AiConversationAttribute) map[string]string {
	out := map[string]string{}
	_ = json.Unmarshal(a.OptionDescriptions, &out)

	return out
}

/*
describe keeps the descriptions sent for a list's options, trimmed and
without empty ones, refusing one of another option or too long.
*/
func describe(options []string, descriptions map[string]string) (json.RawMessage, error) {
	for option := range descriptions {
		if !slices.Contains(options, option) {
			return nil, ErrOptionDescriptions
		}
	}

	return keepDescribed(options, descriptions)
}

/*
keepDescribed keeps the descriptions of the options a list still has,
trimmed and without empty ones, refusing one too long.
*/
func keepDescribed(options []string, descriptions map[string]string) (json.RawMessage, error) {
	out := map[string]string{}

	for option, text := range descriptions {
		text = strings.TrimSpace(text)

		switch {
		case !slices.Contains(options, option):
		case utf8.RuneCountInString(text) > maxOptionDescription:
			return nil, ErrOptionDescriptions
		case text != "":
			out[option] = text
		}
	}

	raw, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("encode option descriptions: %w", err)
	}

	return raw, nil
}

/*
Reorder puts the attributes in the order given, first to last; those
left out keep their place after them.
*/
func (a *Attributes) Reorder(ctx context.Context, m authz.Membership, order []ids.UUID) error {
	if !canManage(m) {
		return exceptions.ErrForbidden
	}

	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, now := models.New(tx), a.Clock.Now()

		for i, id := range order {
			if err := q.MoveConversationAttribute(ctx, models.MoveConversationAttributeParams{
				Position: int32(i - len(order)), //nolint:gosec // a short list
				Now:      now, WorkspaceID: m.Workspace.ID, ID: id,
			}); err != nil {
				return fmt.Errorf("move conversation attribute: %w", err)
			}
		}

		return nil
	})
}

/*
teams lists each team once, an empty list for every team.
*/
func teams(list []ids.UUID) []ids.UUID {
	out := make([]ids.UUID, 0, len(list))
	for _, id := range list {
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}

	return out
}

/*
trimmed trims each choice, an empty list for none.
*/
func trimmed(options []string) []string {
	out := make([]string, 0, len(options))
	for _, o := range options {
		out = append(out, strings.TrimSpace(o))
	}

	return out
}
