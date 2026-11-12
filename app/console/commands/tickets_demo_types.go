//go:build dev

/*
|--------------------------------------------------------------------------
| Tickets Demo Types
|--------------------------------------------------------------------------
|
| The ticket types the demo sets up and their fields. A type found by
| name keeps its settings and only gets the fields it lacks.
|
*/

package commands

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/tickets"
)

/*
demoField is one field of a demo type.
*/
type demoField struct {
	key, label, dataType string
	options              []string
	requiredCreate       bool
	requiredResolve      bool
	customer             bool
	currency             string
	show                 *tickets.ShowWhen
	hint                 string
}

/*
demoType is one type the demo sets up.
*/
type demoType struct {
	name, kind, icon, description string
	fields                        []demoField
}

/*
demoTypes lists the demo's types.
*/
func demoTypes() []demoType {
	return append([]demoType{
		{
			name: "Refund request", kind: tickets.KindCustomer, icon: "refund",
			description: "A customer asks for their money back on an order.", fields: refundFields(),
		},
		{
			name: "Bug report", kind: tickets.KindCustomer,
			fields: []demoField{{
				key: "platform", label: "Platform", dataType: tickets.TypeMultipleChoice,
				options: []string{"Web", "iOS", "Android", "API"}, customer: true,
			}},
		},
	}, moreDemoTypes()...)
}

/*
refundFields are the fields of a refund request.
*/
func refundFields() []demoField {
	return []demoField{
		{
			key: "order_number", label: "Order number", dataType: tickets.TypeShortText, requiredCreate: true,
			customer: true,
		},
		{
			key: "amount", label: "Amount", dataType: tickets.TypeCurrency, currency: "USD", customer: true,
			requiredResolve: true,
		},
		{
			key: "reason", label: "Reason", dataType: tickets.TypeSingleChoice, customer: true,
			options: []string{"Damaged on arrival", "Arrived late", "Wrong item", "Changed my mind", "Other"},
			hint:    "Why the customer wants a refund, from their words.",
		},
		{
			key: "reason_detail", label: "Tell us more", dataType: tickets.TypeLongText, customer: true,
			show: &tickets.ShowWhen{Field: "reason", Equals: []string{"Other"}},
		},
	}
}

/*
moreDemoTypes are the account access type and the back-office types.
*/
func moreDemoTypes() []demoType {
	return []demoType{
		{
			name: "Account access", kind: tickets.KindCustomer, icon: "lock",
			description: "A customer cannot sign in or lost a second factor.",
			fields: []demoField{
				{
					key: "issue", label: "What happened", dataType: tickets.TypeSingleChoice, customer: true,
					requiredCreate: true,
					options:        []string{"Password reset", "Lost 2FA device", "SSO error", "Account locked"},
				},
				{
					key: "identity_provider", label: "Identity provider", dataType: tickets.TypeShortText,
					customer: true,
					show:     &tickets.ShowWhen{Field: "issue", Equals: []string{"SSO error"}},
				},
			},
		},
		{
			name: "Billing", kind: tickets.KindBackOffice, icon: "bank-card",
			description: "Refunds, credits and invoice fixes the billing team makes.",
			fields: []demoField{{
				key: "credit_note", label: "Credit note", dataType: tickets.TypeShortText,
				requiredResolve: true,
			}},
		},
		{
			name: "Engineering", kind: tickets.KindBackOffice, icon: "code",
			description: "Fixes engineering makes for one customer.",
			fields: []demoField{{
				key: "component", label: "Component", dataType: tickets.TypeSingleChoice,
				options: []string{"Exports", "Mobile", "Billing", "Auth", "Messenger"},
			}},
		},
	}
}

/*
everydayStates are the states the demo's new types use, the default
first.
*/
func everydayStates() []string {
	return []string{
		"New", "Investigating", "Close the loop", "Needs info", "With another team", "Being worked on", "Resolved",
		"Won't fix",
	}
}

/*
ensureType adds the type unless one has its name, then the fields it
lacks.
*/
func (d *ticketDemo) ensureType(ctx context.Context, setup tickets.Setup, spec demoType) error {
	id, ok := d.types[spec.name]
	if !ok {
		states := make([]ids.UUID, 0, len(everydayStates()))
		for _, name := range everydayStates() {
			states = append(states, d.states[name])
		}

		def, resolved := d.states["New"], d.states["Resolved"]

		made, err := d.app.Actions.Tickets.CreateType(ctx, d.owner, tickets.TypeInput{
			Kind: &spec.kind, Name: &spec.name, Icon: &spec.icon, Description: &spec.description, States: states,
			DefaultState: &def, DefaultResolved: &resolved,
		})
		if err != nil {
			return err
		}

		id = made.ID
		d.types[spec.name] = id
	}

	have := map[string]bool{}
	for _, f := range setup.FieldsOf(id) {
		have[f.Key] = true
	}

	for _, f := range spec.fields {
		if have[f.key] {
			continue
		}

		if _, err := d.app.Actions.Tickets.CreateField(ctx, d.owner, id, fieldInput(f)); err != nil {
			return err
		}
	}

	return nil
}

/*
fieldInput shapes a demo field for the settings action.
*/
func fieldInput(f demoField) tickets.FieldInput {
	in := tickets.FieldInput{
		Key: &f.key, Label: &f.label, DataType: &f.dataType, Options: f.options, RequiredCreate: &f.requiredCreate,
		RequiredResolve: &f.requiredResolve, VisibleCustomer: &f.customer, EditableCustomer: &f.customer,
	}

	if f.currency != "" {
		currency := &f.currency
		in.Currency = &currency
	}

	if f.show != nil {
		in.ShowWhen = &f.show
	}

	if f.hint != "" {
		hint := &f.hint
		in.AIHint = &hint
	}

	return in
}
