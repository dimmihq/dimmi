/*
|--------------------------------------------------------------------------
| Field Settings
|--------------------------------------------------------------------------
|
| A type's fields: added with a key and data type that never change,
| changed, put in order and archived, which keeps stored values.
|
*/

package tickets

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
FieldInput is a field's settings; nil leaves a field. Key and DataType
are read only when the field is created.
*/
type FieldInput struct {
	Key                    *string
	Label                  *string
	Description            *string
	DataType               *string
	Options                []string
	Currency               **string
	RequiredCreate         *bool
	RequiredCreateCustomer *bool
	RequiredResolve        *bool
	VisibleCustomer        *bool
	EditableCustomer       *bool
	ShowWhen               **ShowWhen
	AIHint                 **string
	AIAuto                 *bool
}

/*
CreateField adds a field at the end of a type's fields.
*/
func (t *Tickets) CreateField(
	ctx context.Context, m authz.Membership, typeID ids.UUID, in FieldInput,
) (models.TicketField, error) {
	var out models.TicketField

	err := t.manage(ctx, m, func(ctx context.Context, w *Writer) error {
		if _, ok := w.Setup.Type(typeID); !ok {
			return ErrTypeNotFound
		}

		if in.Key == nil || !keyShape.MatchString(*in.Key) {
			return ErrKeyInvalid
		}

		if in.DataType == nil || !slices.Contains(DataTypes(), *in.DataType) {
			return ErrDataTypeInvalid
		}

		next, err := applyField(models.TicketField{Key: *in.Key, DataType: *in.DataType, Options: []string{}}, in)
		if err != nil {
			return err
		}

		out, err = w.Q.CreateTicketField(ctx, createFieldParams(m.Workspace.ID, typeID, next, w))
		if isNoRows(err) {
			return ErrKeyTaken
		}

		if err != nil {
			return fmt.Errorf("create ticket field: %w", err)
		}

		return nil
	})

	return out, err
}

/*
createFieldParams shapes a new field for the query.
*/
func createFieldParams(ws, typeID ids.UUID, f models.TicketField, w *Writer) models.CreateTicketFieldParams {
	return models.CreateTicketFieldParams{
		WorkspaceID: ws, TypeID: typeID, Key: f.Key, Label: f.Label, Description: f.Description,
		DataType: f.DataType, Options: f.Options, Currency: f.Currency, RequiredCreate: f.RequiredCreate,
		RequiredCreateCustomer: f.RequiredCreateCustomer, RequiredResolve: f.RequiredResolve,
		VisibleCustomer: f.VisibleCustomer, EditableCustomer: f.EditableCustomer, ShowWhen: f.ShowWhen,
		AiHint: f.AiHint, AiAuto: f.AiAuto, Now: w.Now,
	}
}

/*
UpdateField changes a field's settings; its key and data type stay.
*/
func (t *Tickets) UpdateField(
	ctx context.Context, m authz.Membership, id ids.UUID, in FieldInput,
) (models.TicketField, error) {
	var out models.TicketField

	err := t.manage(ctx, m, func(ctx context.Context, w *Writer) error {
		cur, err := w.Q.FindTicketField(ctx, models.FindTicketFieldParams{WorkspaceID: m.Workspace.ID, ID: id})
		if err != nil || cur.ArchivedAt != nil {
			return ErrFieldNotFound
		}

		next, err := applyField(cur, in)
		if err != nil {
			return err
		}

		out, err = w.Q.UpdateTicketField(ctx, models.UpdateTicketFieldParams{
			Label: next.Label, Description: next.Description, Options: next.Options, Currency: next.Currency,
			RequiredCreate: next.RequiredCreate, RequiredCreateCustomer: next.RequiredCreateCustomer,
			RequiredResolve: next.RequiredResolve, VisibleCustomer: next.VisibleCustomer,
			EditableCustomer: next.EditableCustomer, ShowWhen: next.ShowWhen, AiHint: next.AiHint,
			AiAuto: next.AiAuto, Now: w.Now, WorkspaceID: m.Workspace.ID, ID: id,
		})
		if err != nil {
			return fmt.Errorf("update ticket field: %w", err)
		}

		return nil
	})

	return out, err
}

/*
applyField applies the input to a field and checks it.
*/
func applyField(cur models.TicketField, in FieldInput) (models.TicketField, error) {
	setText(&cur.Label, in.Label)
	setText(&cur.Description, in.Description)
	setOpt(&cur.Currency, in.Currency)
	setValue(&cur.RequiredCreate, in.RequiredCreate)
	setValue(&cur.RequiredCreateCustomer, in.RequiredCreateCustomer)
	setValue(&cur.RequiredResolve, in.RequiredResolve)
	setValue(&cur.VisibleCustomer, in.VisibleCustomer)
	setValue(&cur.EditableCustomer, in.EditableCustomer)
	setOpt(&cur.AiHint, in.AIHint)
	setValue(&cur.AiAuto, in.AIAuto)

	if in.Options != nil {
		cur.Options = in.Options
	}

	if in.ShowWhen != nil {
		cur.ShowWhen = nil
		if *in.ShowWhen != nil {
			raw, _ := json.Marshal(*in.ShowWhen)
			cur.ShowWhen = raw
		}
	}

	if cur.EditableCustomer {
		cur.VisibleCustomer = true
	}

	return cur, checkField(cur)
}

/*
checkField refuses settings a field may not have.
*/
func checkField(f models.TicketField) error {
	choice := f.DataType == TypeSingleChoice || f.DataType == TypeMultipleChoice

	switch {
	case len([]rune(f.Label)) < 1 || len([]rune(f.Label)) > 80:
		return ErrNameInvalid.OnField("label")
	case choice && len(f.Options) == 0:
		return ErrNameInvalid.OnField("options")
	case len(f.Options) > 200:
		return ErrNameInvalid.OnField("options")
	case f.AiHint != nil && len([]rune(*f.AiHint)) > 500:
		return ErrNameInvalid.OnField("ai_hint")
	}

	for _, o := range f.Options {
		if n := len([]rune(o)); n < 1 || n > 100 {
			return ErrNameInvalid.OnField("options")
		}
	}

	return nil
}

/*
ReorderFields saves the order of a type's fields.
*/
func (t *Tickets) ReorderFields(
	ctx context.Context, m authz.Membership, typeID ids.UUID, order []ids.UUID,
) ([]models.TicketField, error) {
	var out []models.TicketField

	err := t.manage(ctx, m, func(ctx context.Context, w *Writer) error {
		var have []ids.UUID
		for _, f := range w.Setup.FieldsOf(typeID) {
			have = append(have, f.ID)
		}

		if len(have) == 0 && len(order) > 0 || !sameSet(have, order) {
			return ErrOrderInvalid
		}

		if err := w.Q.SaveTicketFieldOrder(ctx, models.SaveTicketFieldOrderParams{
			Now: w.Now, Ids: order, WorkspaceID: m.Workspace.ID, TypeID: typeID,
		}); err != nil {
			return fmt.Errorf("order ticket fields: %w", err)
		}

		var err error
		out, err = w.Q.ListTypeFields(ctx, models.ListTypeFieldsParams{WorkspaceID: m.Workspace.ID, TypeID: typeID})

		return err
	})

	return out, err
}

/*
ArchiveField hides a field from forms; stored values stay.
*/
func (t *Tickets) ArchiveField(ctx context.Context, m authz.Membership, id ids.UUID) error {
	return t.manage(ctx, m, func(ctx context.Context, w *Writer) error {
		if _, err := w.Q.FindTicketField(ctx, models.FindTicketFieldParams{
			WorkspaceID: m.Workspace.ID, ID: id,
		}); err != nil {
			return ErrFieldNotFound
		}

		if err := w.Q.ArchiveTicketField(ctx, models.ArchiveTicketFieldParams{
			Now: &w.Now, WorkspaceID: m.Workspace.ID, ID: id,
		}); err != nil {
			return fmt.Errorf("archive ticket field: %w", err)
		}

		return nil
	})
}

/*
TypeOfField returns the type a field belongs to, for the API's paths.
*/
func TypeOfField(fields []models.TicketField, id ids.UUID) (ids.UUID, bool) {
	i := slices.IndexFunc(fields, func(f models.TicketField) bool { return f.ID == id })
	if i < 0 {
		return ids.Nil(), false
	}

	return fields[i].TypeID, true
}
