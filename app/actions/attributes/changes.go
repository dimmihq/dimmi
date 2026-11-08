/*
|--------------------------------------------------------------------------
| Changing Attribute Definitions
|--------------------------------------------------------------------------
|
| A definition's label, description and list values may change; its key
| and type never do. Letting browsers write a contact attribute adds it
| to the sign-in settings' list, the one place the messenger reads, which
| holds 20 keys at most. Problems list the writes that were dropped in
| the last 30 days.
|
*/

package attributes

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/identity"
	"github.com/dimmihq/dimmi/app/attributedefs"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxWritable is how many contact attributes browsers may write.
*/
const maxWritable = 20

/*
ErrTooManyWritable refuses a 21st attribute browsers may write.
*/
var ErrTooManyWritable = fault.New(fault.Invalid, "writable_limit_reached",
	"Browsers may write 20 attributes at most.").OnField("client_writable")

/*
UpdateInput changes a definition. Nil fields stay as they are.
*/
type UpdateInput struct {
	Membership     authz.Membership
	ID             ids.UUID
	Label          *string
	Description    *string
	Options        *[]string
	Status         *string
	ClientWritable *bool
}

/*
Update changes a definition.
*/
func (d *Definitions) Update(ctx context.Context, in UpdateInput) (Definition, error) {
	m := in.Membership
	if !m.Can("attributes:manage") {
		return Definition{}, exceptions.ErrForbidden
	}

	var out Definition
	err := d.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, now := models.New(tx), d.Clock.Now()
		row, err := find(ctx, q, m.Workspace.ID, in.ID)
		if err != nil {
			return err
		}

		params, err := changed(ctx, q, row, in)
		if err != nil {
			return err
		}

		params.Now = now
		if row, err = q.UpdateAttributeDefinition(ctx, params); err != nil {
			return fmt.Errorf("update attribute definition: %w", err)
		}

		on, err := setWritable(ctx, q, writable{row: row, member: m.Member.ID, want: in.ClientWritable})
		out = Definition{AttributeDefinition: row, ClientWritable: on}

		return err
	})

	return out, err
}

/*
changed applies the input over the stored definition and checks it.
*/
func changed(ctx context.Context, q *models.Queries, row models.AttributeDefinition, in UpdateInput,
) (models.UpdateAttributeDefinitionParams, error) {
	p := models.UpdateAttributeDefinitionParams{
		Label: row.Label, Description: row.Description, Options: row.Options, Status: row.Status,
		WorkspaceID: row.WorkspaceID, ID: row.ID,
	}

	if in.Label != nil {
		p.Label = strings.TrimSpace(*in.Label)
		if n := utf8.RuneCountInString(p.Label); n < 1 || n > 80 {
			return p, ErrLabel
		}
	}

	if in.Description != nil {
		p.Description = in.Description
	}

	if in.Options != nil {
		if err := checkOptions(row.DataType, *in.Options); err != nil {
			return p, err
		}

		p.Options, _ = json.Marshal(*in.Options)
	}

	if in.Status == nil || *in.Status == row.Status {
		return p, nil
	}

	p.Status = *in.Status
	if p.Status == "active" {
		return p, nil
	}

	if p.Status != "archived" {
		return p, fault.New(fault.Invalid, "status_invalid", "Pick active or archived.").OnField("status")
	}

	return p, unused(ctx, q, row)
}

/*
unused refuses to stop an attribute that audiences or workflows read.
*/
func unused(ctx context.Context, q *models.Queries, row models.AttributeDefinition) error {
	users, err := q.AttributeUsers(ctx, models.AttributeUsersParams{
		WorkspaceID: row.WorkspaceID, Field: row.Object + ".custom." + row.Key,
	})
	if err != nil {
		return fmt.Errorf("list attribute users: %w", err)
	}

	if len(users) == 0 {
		return nil
	}

	names := make([]string, 0, len(users))
	for _, u := range users {
		names = append(names, u.Name)
	}

	return ErrInUse.With("used_by", names)
}

/*
writable is a change to whether browsers may write an attribute: the
attribute, who changes it, and nil to leave it as it is.
*/
type writable struct {
	row    models.AttributeDefinition
	member ids.UUID
	want   *bool
}

/*
setWritable lets browsers write a contact attribute, or stops them, and
says whether they may now.
*/
func setWritable(ctx context.Context, q *models.Queries, w writable) (bool, error) {
	row, want := w.row, w.want
	settings, err := identity.LoadSettings(ctx, q, row.WorkspaceID)
	if err != nil {
		return false, err
	}

	keys := slices.DeleteFunc(slices.Clone(settings.ClientWritableAttributes), func(k string) bool {
		return k == row.Key
	})
	now := slices.Contains(settings.ClientWritableAttributes, row.Key)
	if want == nil || *want == now || row.Object != attributedefs.ObjectContact {
		return now && row.Object == attributedefs.ObjectContact, nil
	}

	if *want {
		if len(keys) >= maxWritable {
			return now, ErrTooManyWritable
		}

		keys = append(keys, row.Key)
	}

	slices.Sort(keys)
	_, err = q.UpsertIdentitySettings(ctx, models.UpsertIdentitySettingsParams{
		WorkspaceID: row.WorkspaceID, Mode: settings.Mode, ClientWritableAttributes: keys,
		UpdatedByMemberID: &w.member, Now: row.UpdatedAt,
	})
	if err != nil {
		return now, fmt.Errorf("save writable attributes: %w", err)
	}

	return *want, nil
}

/*
writableKeys lists the contact attributes browsers may write.
*/
func writableKeys(ctx context.Context, q *models.Queries, ws ids.UUID) ([]string, error) {
	settings, err := identity.LoadSettings(ctx, q, ws)

	return settings.ClientWritableAttributes, err
}

/*
Delete removes an archived definition and every value of it.
*/
func (d *Definitions) Delete(ctx context.Context, m authz.Membership, id ids.UUID) error {
	if !m.Can("attributes:manage") {
		return exceptions.ErrForbidden
	}

	return d.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		row, err := find(ctx, q, m.Workspace.ID, id)
		if err != nil {
			return err
		}

		if row.Status != "archived" {
			return ErrActive
		}

		if err := removeValues(ctx, q, row); err != nil {
			return err
		}

		if _, err := setWritable(ctx, q, writable{row: row, member: m.Member.ID, want: new(false)}); err != nil {
			return err
		}

		return wrap("delete attribute definition", q.DeleteAttributeDefinition(ctx,
			models.DeleteAttributeDefinitionParams{WorkspaceID: row.WorkspaceID, ID: row.ID}))
	})
}

/*
Problems lists the writes dropped in the last 30 days.
*/
func (d *Definitions) Problems(ctx context.Context, m authz.Membership) ([]models.AttributeProblem, error) {
	if !m.Can("contacts:read") {
		return nil, exceptions.ErrForbidden
	}

	var out []models.AttributeProblem
	err := d.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = models.New(tx).ListAttributeProblems(ctx, models.ListAttributeProblemsParams{
			WorkspaceID: m.Workspace.ID, Since: d.Clock.Now().Add(-30 * 24 * time.Hour),
		})

		return wrap("list attribute problems", err)
	})

	return out, err
}

/*
removeValues removes an attribute's key from every record holding it.
*/
func removeValues(ctx context.Context, q *models.Queries, row models.AttributeDefinition) error {
	var err error
	if row.Object == attributedefs.ObjectContact {
		err = q.RemoveContactAttribute(ctx, models.RemoveContactAttributeParams{
			WorkspaceID: row.WorkspaceID, Key: row.Key,
		})
	} else {
		err = q.RemoveCompanyAttribute(ctx, models.RemoveCompanyAttributeParams{
			WorkspaceID: row.WorkspaceID, Key: row.Key,
		})
	}

	return wrap("remove attribute values", err)
}
