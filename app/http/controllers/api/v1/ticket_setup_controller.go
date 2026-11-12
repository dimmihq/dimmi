/*
|--------------------------------------------------------------------------
| Ticket Setup Controller
|--------------------------------------------------------------------------
|
| How a workspace sets tickets up: its types with their fields and the
| states they use, the fields of each type in order, and the states in
| their five groups. Removing a state names where its tickets go;
| archiving a type or field keeps it on old tickets.
|
*/

package v1

import (
	"context"
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
TicketSetupController serves types, fields and states.
*/
type TicketSetupController struct {
	TicketController
}

/*
TicketTypesOutput is every type.
*/
type TicketTypesOutput struct {
	Body fhttp.Resource[[]resources.TicketType]
}

/*
TicketTypeOutput is one type.
*/
type TicketTypeOutput struct {
	Body fhttp.Resource[resources.TicketType]
}

/*
Types lists the types with their fields and states.
*/
func (c *TicketSetupController) Types(ctx context.Context, _ *requests.NoInput) (*TicketTypesOutput, error) {
	setup, counts, err := c.Tickets.Settings(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}

	return &TicketTypesOutput{Body: fhttp.NewResource(resources.NewTicketTypes(setup, counts))}, nil
}

/*
ShowType answers one type.
*/
func (c *TicketSetupController) ShowType(ctx context.Context, in *requests.TicketTypeRequest,
) (*TicketTypeOutput, error) {
	id, err := c.decodeID(models.PrefixTicketType, in.Type)
	if err != nil {
		return nil, err
	}

	return c.typeOutput(ctx, id)
}

/*
typeOutput answers a type as it is now.
*/
func (c *TicketSetupController) typeOutput(ctx context.Context, id ids.UUID) (*TicketTypeOutput, error) {
	setup, counts, err := c.Tickets.Settings(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}

	t, ok := setup.Type(id)
	if !ok {
		return nil, tickets.ErrTypeNotFound
	}

	return &TicketTypeOutput{Body: fhttp.NewResource(resources.NewTicketType(t, setup, counts))}, nil
}

/*
decodeID reads one public id or answers not found.
*/
func (c *TicketSetupController) decodeID(prefix models.Prefix, s string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(prefix), s)
	if err != nil {
		return id, exceptions.ErrNotFound
	}

	return id, nil
}

/*
StoreType adds a type.
*/
func (c *TicketSetupController) StoreType(ctx context.Context, in *requests.StoreTicketTypeRequest,
) (*TicketTypeOutput, error) {
	input, err := c.typeInput(in.Body, map[string]bool{})
	if err != nil {
		return nil, err
	}

	t, err := c.Tickets.CreateType(ctx, membership(ctx), input)
	if err != nil {
		return nil, err
	}

	return c.typeOutput(ctx, t.ID)
}

/*
UpdateType changes a type.
*/
func (c *TicketSetupController) UpdateType(ctx context.Context, in *requests.UpdateTicketTypeRequest,
) (*TicketTypeOutput, error) {
	id, err := c.decodeID(models.PrefixTicketType, in.Type)
	if err != nil {
		return nil, err
	}

	input, err := c.typeInput(in.Body, requests.Sent(in.RawBody))
	if err != nil {
		return nil, err
	}

	if _, err := c.Tickets.UpdateType(ctx, membership(ctx), id, input); err != nil {
		return nil, err
	}

	return c.typeOutput(ctx, id)
}

/*
typeInput reads a type's settings; sent says which keys the body named,
so a null clears.
*/
func (c *TicketSetupController) typeInput(b requests.TicketTypeBody, sent map[string]bool) (tickets.TypeInput, error) {
	out := tickets.TypeInput{
		Kind: b.Kind, Name: b.Name, Icon: b.Icon, Description: b.Description, CustomerVisible: b.CustomerVisible,
		CompanySharing: b.CompanySharing, ReopenDays: b.ReopenDays,
		AIAutoFill: b.AIAutoFill,
	}

	if b.AutoResolveDays != nil || sent["auto_resolve_days"] {
		out.AutoResolveDays = &b.AutoResolveDays
	}

	if b.AutoResolveMessage != nil || sent["auto_resolve_message"] {
		out.AutoResolveText = &b.AutoResolveMessage
	}

	var err error
	if out.InboxID, err = c.clearable(models.PrefixInbox, b.InboxID, sentKey{sent, "inbox_id"}); err != nil {
		return out, err
	}

	if out.TeamID, err = c.clearable(models.PrefixTeam, b.TeamID, sentKey{sent, "team_id"}); err != nil {
		return out, err
	}

	err = c.typeStates(b, &out)

	return out, err
}

/*
sentKey names a body key and which keys the body sent.
*/
type sentKey struct {
	sent map[string]bool
	key  string
}

/*
clearable reads an id the body may set or clear with null.
*/
func (c *TicketSetupController) clearable(prefix models.Prefix, s *string, k sentKey) (**ids.UUID, error) {
	if s == nil && !k.sent[k.key] {
		return nil, nil //nolint:nilnil // left out stays
	}

	id, err := c.optionalPtr(prefix, s)

	return &id, err
}

/*
typeStates reads the states a type may use and its defaults.
*/
func (c *TicketSetupController) typeStates(b requests.TicketTypeBody, out *tickets.TypeInput) error {
	if b.StateIDs != nil {
		list, err := c.idList(models.PrefixTicketState, joinIDs(b.StateIDs))
		if err != nil {
			return err
		}

		out.States = append([]ids.UUID{}, list...)
	}

	var err error
	if out.DefaultState, err = c.optionalPtr(models.PrefixTicketState, b.DefaultStateID); err != nil {
		return err
	}

	out.DefaultResolved, err = c.optionalPtr(models.PrefixTicketState, b.DefaultResolvedID)

	return err
}

/*
joinIDs writes ids comma-separated.
*/
func joinIDs(list []string) string {
	var out strings.Builder
	for i, s := range list {
		if i > 0 {
			out.WriteString(",")
		}

		out.WriteString(s)
	}

	return out.String()
}

/*
DestroyType archives a type.
*/
func (c *TicketSetupController) DestroyType(ctx context.Context, in *requests.TicketTypeRequest) (*struct{}, error) {
	id, err := c.decodeID(models.PrefixTicketType, in.Type)
	if err != nil {
		return nil, err
	}

	return &struct{}{}, c.Tickets.ArchiveType(ctx, membership(ctx), id)
}

/*
Fields lists a type's fields.
*/
func (c *TicketSetupController) Fields(ctx context.Context, in *requests.TicketTypeRequest,
) (*TicketFieldsOutput, error) {
	id, err := c.decodeID(models.PrefixTicketType, in.Type)
	if err != nil {
		return nil, err
	}

	setup, _, err := c.Tickets.Settings(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}

	if _, ok := setup.Type(id); !ok {
		return nil, tickets.ErrTypeNotFound
	}

	return &TicketFieldsOutput{Body: fhttp.NewResource(resources.NewTicketFields(setup.FieldsOf(id)))}, nil
}

/*
StoreField adds a field to a type.
*/
func (c *TicketSetupController) StoreField(ctx context.Context, in *requests.StoreTicketFieldRequest,
) (*TicketFieldOutput, error) {
	id, err := c.decodeID(models.PrefixTicketType, in.Type)
	if err != nil {
		return nil, err
	}

	f, err := c.Tickets.CreateField(ctx, membership(ctx), id, fieldInput(in.Body, map[string]bool{}))
	if err != nil {
		return nil, err
	}

	return &TicketFieldOutput{Body: fhttp.NewResource(resources.NewTicketField(f))}, nil
}

/*
UpdateField changes a field.
*/
func (c *TicketSetupController) UpdateField(ctx context.Context, in *requests.UpdateTicketFieldRequest,
) (*TicketFieldOutput, error) {
	id, err := c.decodeID(models.PrefixTicketField, in.Field)
	if err != nil {
		return nil, err
	}

	input := fieldInput(in.Body, requests.Sent(in.RawBody))
	input.Key, input.DataType = nil, nil

	f, err := c.Tickets.UpdateField(ctx, membership(ctx), id, input)
	if err != nil {
		return nil, err
	}

	return &TicketFieldOutput{Body: fhttp.NewResource(resources.NewTicketField(f))}, nil
}

/*
fieldInput reads a field's settings; sent says which keys the body
named, so a null clears.
*/
func fieldInput(b requests.TicketFieldBody, sent map[string]bool) tickets.FieldInput {
	out := tickets.FieldInput{
		Key: b.Key, Label: b.Label, Description: b.Description, DataType: b.DataType, Options: b.Options,
		RequiredCreate: b.RequiredCreate, RequiredCreateCustomer: b.RequiredCreateCustomer,
		RequiredResolve: b.RequiredResolve, VisibleCustomer: b.VisibleCustomer, EditableCustomer: b.EditableCustomer,
		AIAuto: b.AIAuto,
	}

	if b.Currency != nil || sent["currency"] {
		out.Currency = &b.Currency
	}

	if b.AIHint != nil || sent["ai_hint"] {
		out.AIHint = &b.AIHint
	}

	if b.ShowWhen != nil || sent["show_when"] {
		var sw *tickets.ShowWhen
		if b.ShowWhen != nil {
			sw = &tickets.ShowWhen{Field: b.ShowWhen.Field, Equals: b.ShowWhen.Equals}
		}

		out.ShowWhen = &sw
	}

	return out
}

/*
ReorderFields saves the order of a type's fields.
*/
func (c *TicketSetupController) ReorderFields(ctx context.Context, in *requests.ReorderTicketFieldsRequest,
) (*TicketFieldsOutput, error) {
	id, err := c.decodeID(models.PrefixTicketType, in.Type)
	if err != nil {
		return nil, err
	}

	order, err := c.idList(models.PrefixTicketField, joinIDs(in.Body.FieldIDs))
	if err != nil {
		return nil, err
	}

	list, err := c.Tickets.ReorderFields(ctx, membership(ctx), id, order)
	if err != nil {
		return nil, err
	}

	return &TicketFieldsOutput{Body: fhttp.NewResource(resources.NewTicketFields(list))}, nil
}

/*
DestroyField archives a field.
*/
func (c *TicketSetupController) DestroyField(ctx context.Context, in *requests.TicketFieldRequest,
) (*struct{}, error) {
	id, err := c.decodeID(models.PrefixTicketField, in.Field)
	if err != nil {
		return nil, err
	}

	return &struct{}{}, c.Tickets.ArchiveField(ctx, membership(ctx), id)
}

/*
States lists the states, group by group.
*/
func (c *TicketSetupController) States(ctx context.Context, _ *requests.NoInput) (*TicketStatesOutput, error) {
	setup, _, err := c.Tickets.Settings(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}

	return &TicketStatesOutput{Body: fhttp.NewResource(resources.NewTicketStates(setup.States))}, nil
}

/*
StoreState adds a state.
*/
func (c *TicketSetupController) StoreState(ctx context.Context, in *requests.StoreTicketStateRequest,
) (*TicketStateOutput, error) {
	s, err := c.Tickets.CreateState(ctx, membership(ctx), stateInput(in.Body))
	if err != nil {
		return nil, err
	}

	return &TicketStateOutput{Body: fhttp.NewResource(resources.NewTicketState(s))}, nil
}

/*
stateInput reads a state's settings.
*/
func stateInput(b requests.TicketStateBody) tickets.StateInput {
	return tickets.StateInput{
		Group: b.Group, Name: b.Name, CustomerLabel: b.CustomerLabel, NotifyCustomer: b.NotifyCustomer,
	}
}

/*
UpdateState changes a state.
*/
func (c *TicketSetupController) UpdateState(ctx context.Context, in *requests.UpdateTicketStateRequest,
) (*TicketStateOutput, error) {
	id, err := c.decodeID(models.PrefixTicketState, in.State)
	if err != nil {
		return nil, err
	}

	input := stateInput(in.Body)
	input.Group = nil

	s, err := c.Tickets.UpdateState(ctx, membership(ctx), id, input)
	if err != nil {
		return nil, err
	}

	return &TicketStateOutput{Body: fhttp.NewResource(resources.NewTicketState(s))}, nil
}

/*
ReorderStates saves the order of the states.
*/
func (c *TicketSetupController) ReorderStates(ctx context.Context, in *requests.ReorderTicketStatesRequest,
) (*TicketStatesOutput, error) {
	order, err := c.idList(models.PrefixTicketState, joinIDs(in.Body.StateIDs))
	if err != nil {
		return nil, err
	}

	list, err := c.Tickets.ReorderStates(ctx, membership(ctx), order)
	if err != nil {
		return nil, err
	}

	return &TicketStatesOutput{Body: fhttp.NewResource(resources.NewTicketStates(list))}, nil
}

/*
DestroyState removes a state, moving its tickets.
*/
func (c *TicketSetupController) DestroyState(ctx context.Context, in *requests.DestroyTicketStateRequest,
) (*struct{}, error) {
	id, err := c.decodeID(models.PrefixTicketState, in.State)
	if err != nil {
		return nil, err
	}

	var target *ids.UUID
	if err := c.optional(models.PrefixTicketState, in.MoveTo, &target); err != nil {
		return nil, tickets.ErrMoveToRequired
	}

	return &struct{}{}, c.Tickets.DeleteState(ctx, membership(ctx), id, target)
}
