/*
|--------------------------------------------------------------------------
| Bulk Member Changes
|--------------------------------------------------------------------------
|
| PATCH /members applies one change to up to a hundred members, such as
| moving everyone off a custom role before it is deleted. Each member is
| changed on its own, through the same rules as a single change, and the
| answer says for each one whether it was updated, skipped because it
| already stood that way, or failed and why. Partial success is allowed,
| never silent.
|
*/

package v1

import (
	"context"
	"encoding/json"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/validation"

	"github.com/dimmihq/dimmi/app/actions/members"
	"github.com/dimmihq/dimmi/app/actions/roles"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Outcomes of one member's change in a bulk request.
*/
const (
	bulkUpdated = "updated"
	bulkSkipped = "skipped"
	bulkFailed  = "failed"
)

/*
BulkResult is what one member's change came to, with the reason when it
failed.
*/
type BulkResult struct {
	ID     string     `json:"id"`
	Status string     `json:"status"`
	Error  *BulkError `json:"error,omitempty"`
}

/*
BulkError is why one change failed.
*/
type BulkError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

/*
BulkMeta counts the outcomes.
*/
type BulkMeta struct {
	Updated int `json:"updated"`
	Skipped int `json:"skipped"`
	Failed  int `json:"failed"`
}

/*
BulkMembersOutput is the outcome of each member's change.
*/
type BulkMembersOutput struct {
	Body struct {
		Data []BulkResult `json:"data"`
		Meta BulkMeta     `json:"meta"`
	}
}

/*
UpdateMany applies the change to each member named and answers with
what each came to.
*/
func (c *MemberController) UpdateMany(
	ctx context.Context, in *requests.UpdateMembersRequest,
) (*BulkMembersOutput, error) {
	change, err := c.bulkChange(in)
	if err != nil {
		return nil, err
	}

	out := &BulkMembersOutput{}
	out.Body.Data = make([]BulkResult, 0, len(in.Body.IDs))

	for _, named := range in.Body.IDs {
		result := c.changeOne(ctx, named, change)
		out.Body.Data = append(out.Body.Data, result)

		switch result.Status {
		case bulkUpdated:
			out.Body.Meta.Updated++
		case bulkSkipped:
			out.Body.Meta.Skipped++
		default:
			out.Body.Meta.Failed++
		}
	}

	return out, nil
}

/*
bulkChange is the change every member gets: a built-in role, or a
custom role, nil to take it away.
*/
type bulkChange struct {
	role       *enums.Role
	customRole bool
	roleID     *ids.UUID
}

/*
bulkChange reads the change the request asks for, refusing one that
names both kinds of role, or neither.
*/
func (c *MemberController) bulkChange(in *requests.UpdateMembersRequest) (bulkChange, error) {
	changes := in.Body.Changes
	custom := requests.Sent(changesOf(in.RawBody))["custom_role_id"]

	if (changes.AssignedRole != nil) == custom {
		return bulkChange{}, &validation.Error{Violations: []validation.Violation{{
			Field: "changes", Rule: "required_one", Message: "Send either role or custom_role_id in changes.",
		}}}
	}

	if !custom {
		return bulkChange{role: (*enums.Role)(changes.AssignedRole)}, nil
	}

	out := bulkChange{customRole: true}

	if changes.CustomRoleID != nil {
		id, err := roleID(c.IDs, *changes.CustomRoleID)
		if err != nil {
			return bulkChange{}, err
		}

		out.roleID = &id
	}

	return out, nil
}

/*
changeOne applies the change to one member and says what it came to.
*/
func (c *MemberController) changeOne(ctx context.Context, named string, change bulkChange) BulkResult {
	result := BulkResult{ID: named, Status: bulkUpdated}

	id, err := c.IDs.Decode(string(models.PrefixMember), named)
	if err != nil {
		return failed(ctx, result, exceptions.ErrNotFound)
	}

	before, err := c.ShowMember.Handle(ctx, members.ShowMemberInput{Membership: membership(ctx), MemberID: id})
	if err != nil {
		return failed(ctx, result, err)
	}

	if change.unchanged(before.WorkspaceMember) {
		result.Status = bulkSkipped

		return result
	}

	if change.customRole {
		_, err = c.AssignRole.Handle(ctx, roles.AssignRoleInput{
			Membership: membership(ctx), MemberID: id, RoleID: change.roleID,
		})
	} else {
		_, err = c.UpdateMember.Handle(ctx, members.UpdateMemberInput{
			Principal: caller(ctx), Membership: membership(ctx), MemberID: id, AssignedRole: change.role,
		})
	}

	if err != nil {
		return failed(ctx, result, err)
	}

	return result
}

/*
unchanged reports whether the member already stands as the change asks.
*/
func (b bulkChange) unchanged(m models.WorkspaceMember) bool {
	if !b.customRole {
		return string(*b.role) == m.Role && m.CustomRoleID == nil
	}

	if b.roleID == nil || m.CustomRoleID == nil {
		return b.roleID == nil && m.CustomRoleID == nil
	}

	return *b.roleID == *m.CustomRoleID
}

/*
failed marks the result failed with the error's code and message, in the
reader's language when the code has one.
*/
func failed(ctx context.Context, result BulkResult, err error) BulkResult {
	res := fhttp.Errors{}.Resolve(ctx, err)
	result.Status = bulkFailed
	result.Error = &BulkError{Code: res.Code, Message: res.Message}

	return result
}

/*
changesOf returns the raw changes object of a bulk body, nil when it
has none.
*/
func changesOf(raw []byte) []byte {
	var body struct {
		Changes json.RawMessage `json:"changes"`
	}

	_ = json.Unmarshal(raw, &body)

	return body.Changes
}
