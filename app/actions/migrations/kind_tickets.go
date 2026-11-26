/*
|--------------------------------------------------------------------------
| Ticket Types And Tickets
|--------------------------------------------------------------------------
|
| Ticket types, their fields and states go through the same writer as
| every ticket import, matched to what the workspace already has. A
| type's states are kept in the ledger as "<type>/<state>", so tickets
| name them within their type. A ticket is created at its own time in
| the state it started in, then moved through each later state at the
| time it moved, as the teammate who made it. A later sync moves it to
| the state it is in now, unless a teammate moved it here since.
|
*/

package migrations

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/ticketimports"
	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The ledger kinds of a ticket type's fields; states are kindTicketState.
*/
const kindTicketField = "ticket_field"

/*
Ticket types.
*/
func (t *TicketType) validate() error {
	if err := oneOf("kind", t.Kind, tickets.KindCustomer, tickets.KindBackOffice, tickets.KindTracker); err != nil {
		return err
	}

	if err := required("kind", t.Kind); err != nil {
		return err
	}

	if err := required("name", t.Name); err != nil {
		return err
	}

	if len(t.States) == 0 {
		return invalid("states", "A ticket type has at least one state.")
	}

	for i, st := range t.States {
		if err := required(fmt.Sprintf("states[%d].source_id", i), st.SourceID); err != nil {
			return err
		}

		if err := oneOf(fmt.Sprintf("states[%d].group", i), st.Group, tickets.Groups()...); err != nil {
			return err
		}

		if err := required(fmt.Sprintf("states[%d].group", i), st.Group); err != nil {
			return err
		}
	}

	for i, f := range t.Fields {
		if err := required(fmt.Sprintf("fields[%d].key", i), f.Key); err != nil {
			return err
		}
	}

	return nil
}

func (t *TicketType) refs() []Ref { return nil }

func (t *TicketType) apply(ctx context.Context, s *step) (outcome, error) {
	out := outcome{label: t.Name}

	w := s.ticketWriter(nil)
	catalog := ticketimports.Catalog{Types: []ticketimports.Type{{
		ExternalID: t.SourceID, Kind: t.Kind, Name: t.Name, Description: t.Description,
	}}}

	for _, st := range t.States {
		id := t.SourceID + "/" + st.SourceID
		catalog.States = append(catalog.States, ticketimports.State{
			ExternalID: id, Group: st.Group, Name: st.Name, CustomerLabel: st.CustomerLabel,
		})
		catalog.Types[0].States = append(catalog.Types[0].States, id)
	}

	for _, f := range t.Fields {
		catalog.Types[0].Fields = append(catalog.Types[0].Fields, ticketimports.Field{
			ExternalID: f.SourceID, Key: f.Key, Label: f.Label, DataType: ticketFieldType(f.Type), Options: f.Options,
			Customer: f.CustomerVisible,
		})
	}

	if err := w.MapSetup(ctx, s.tx, catalog); err != nil {
		return out, err
	}

	ty, ok := w.Mapping.Types[t.SourceID]
	if !ok {
		return out, invalid("name", "This ticket type could not be made.")
	}

	out.id = ty.ID
	out.created = s.ledger.typeCreated

	return out, nil
}

/*
ticketFieldType returns the ticket field type for a format type, short
text for one tickets do not know.
*/
func ticketFieldType(t string) string {
	switch t {
	case "string", "":
		return "short_text"
	case "text":
		return "long_text"
	case "list":
		return "single_choice"
	}

	known := []string{
		"short_text", "long_text", "number", "decimal", "currency", "boolean", "date", "single_choice",
		"multiple_choice",
	}
	if slices.Contains(known, t) {
		return t
	}

	return "short_text"
}

/*
ticketLedger keeps the states and fields a ticket writer made or found
in the migration's ledger, and notes whether it made the type.
*/
type ticketLedger struct {
	s           *step
	typeCreated bool
}

/*
Keep keeps one record the writer made or found.
*/
func (l *ticketLedger) Keep(ctx context.Context, q *models.Queries, k ticketimports.Kept) error {
	switch k.Kind {
	case "type":
		l.typeCreated = l.typeCreated || k.Created

		return nil
	case "state":
		return l.s.keepRow(ctx, q, kept{kind: kindTicketState, sourceID: k.External, id: k.ID, created: k.Created})
	case "field":
		return l.s.keepRow(ctx, q, kept{kind: kindTicketField, sourceID: k.External, id: k.ID, created: k.Created})
	}

	return nil
}

/*
kept is a record a step made or found besides its own: its ledger
kind, source id, row, and whether the step made it.
*/
type kept struct {
	kind, sourceID string
	id             ids.UUID
	created        bool
}

/*
keepRow keeps a record a step made besides its own, applied.
*/
func (s *step) keepRow(ctx context.Context, q *models.Queries, k kept) error {
	kind, sourceID, id, created := k.kind, k.sourceID, k.id, k.created
	_, err := q.SaveMigrationRecord(ctx, models.SaveMigrationRecordParams{
		WorkspaceID: s.ws(), MigrationID: s.runner.row.ID, Kind: kind, SourceID: cut(sourceID, 255),
		Status: statusApplied, RecordID: &id, Created: created, WaitingFor: json.RawMessage(`[]`),
		State: json.RawMessage(`{}`), Warnings: json.RawMessage(`[]`), Conflicts: json.RawMessage(`[]`), Now: s.now,
	})
	if err != nil {
		return fmt.Errorf("keep %s: %w", kind, err)
	}

	return nil
}

/*
ticketWriter returns the ticket writer of the step, acting as the
teammate named or the migration.
*/
func (s *step) ticketWriter(author *Author) *ticketimports.Writer {
	s.ledger = &ticketLedger{s: s}

	w := &ticketimports.Writer{
		Clock: s.runner.svc.Clock, Events: s.kit().Events, Tickets: s.kit().Tickets, WorkspaceID: s.ws(),
		Actor: "Migration", Ledger: s.ledger,
	}

	if member, ok := s.member(author); ok {
		w.ActorID, w.Actor = &member, s.refs.label(KindTeammate, author.Ref)
	} else if name := s.authorName(author); name != "" {
		w.Actor = name
	}

	return w
}

/*
Tickets.
*/
func (t *Ticket) validate() error {
	for field, v := range map[string]string{
		"ticket_type_ref": t.TicketTypeRef, "state_ref": t.StateRef, "title": t.Title,
	} {
		if err := required(field, v); err != nil {
			return err
		}
	}

	if t.CreatedAt.IsZero() {
		return invalid("created_at", "created_at is required.")
	}

	if err := oneOf("priority", t.Priority, "low", "medium", "high", "urgent"); err != nil {
		return err
	}

	if t.Author != nil {
		return checkAuthor("author", *t.Author)
	}

	return nil
}

func (t *Ticket) refs() []Ref {
	out := refsOf(KindTicketType, t.TicketTypeRef)
	out = append(out, refsOf(kindTicketState, t.stateKey(t.StateRef))...)

	for _, m := range t.StateHistory {
		out = append(out, refsOf(kindTicketState, t.stateKey(m.StateRef))...)
	}

	out = append(out, refsOf(KindContact, t.ContactRef)...)
	out = append(out, refsOf(KindTeammate, t.AssigneeRef)...)
	out = append(out, refsOf(KindConversation, t.ConversationRef)...)

	return append(out, authorRefs(t.Author)...)
}

/*
stateKey is the ledger source id of one of the ticket's type's states.
*/
func (t *Ticket) stateKey(state string) string {
	if state == "" {
		return ""
	}

	return t.TicketTypeRef + "/" + state
}

/*
ticketState is what a sync of a ticket remembers: the state the
migration last left it in.
*/
type ticketState struct {
	StateID ids.UUID `json:"state_id"`
}

func (t *Ticket) apply(ctx context.Context, s *step) (outcome, error) {
	out := outcome{label: t.Title}

	w := s.ticketWriter(t.Author)

	typeID, err := s.refs.must("ticket_type_ref", KindTicketType, t.TicketTypeRef)
	if err != nil {
		return out, err
	}

	tw, err := w.TicketWriter(ctx, s.tx, t.CreatedAt)
	if err != nil {
		return out, err
	}

	i := slices.IndexFunc(tw.Setup.Types, func(ty models.TicketType) bool { return ty.ID == typeID })
	if i < 0 {
		return out, ErrReferenceUnavailable.OnField("ticket_type_ref")
	}

	w.Mapping.Types = map[string]models.TicketType{t.TicketTypeRef: tw.Setup.Types[i]}
	w.Mapping.States = map[string]ids.UUID{}

	for _, ref := range s.refs.asked {
		if id, ok := s.refs.id(kindTicketState, ref.SourceID); ok && ref.Kind == kindTicketState {
			w.Mapping.States[ref.SourceID] = id
		}
	}

	if s.prev != nil && s.prev.RecordID != nil {
		return s.syncTicket(ctx, w, t, out)
	}

	made, created, err := w.Write(ctx, s.tx, s.ticketInput(t))
	if err != nil {
		return out, err
	}

	out.id, out.created = made.ID, created
	out.state = mustJSON(ticketState{StateID: made.StateID})

	return out, nil
}

/*
ticketInput returns the ticket in the writer's terms.
*/
func (s *step) ticketInput(t *Ticket) ticketimports.Ticket {
	in := ticketimports.Ticket{
		ExternalID: t.SourceID, TypeID: t.TicketTypeRef, StateID: t.stateKey(t.StateRef), Title: t.Title,
		Description: t.Description, Values: t.Values, Priority: t.Priority, CreatedAt: t.CreatedAt.UTC(),
		UpdatedAt: timeOr(t.UpdatedAt, t.CreatedAt),
	}

	for _, m := range t.StateHistory {
		in.History = append(in.History, ticketimports.Move{At: m.At.UTC(), StateID: t.stateKey(m.StateRef)})
	}

	if id, ok := s.refs.id(KindContact, t.ContactRef); ok {
		in.ContactID = &id
	}

	if id, ok := s.refs.id(KindTeammate, t.AssigneeRef); ok {
		in.AssigneeID = &id
	}

	if id, ok := s.refs.id(KindConversation, t.ConversationRef); ok {
		in.ConversationID = &id
	}

	return in
}

/*
syncTicket moves a ticket brought in before to the state it is in now,
unless a teammate moved it here since the migration last did.
*/
func (s *step) syncTicket(ctx context.Context, w *ticketimports.Writer, t *Ticket, out outcome) (outcome, error) {
	current, err := s.q.FindTicket(ctx, models.FindTicketParams{WorkspaceID: s.ws(), ID: *s.prev.RecordID})
	if err != nil || current.DeletedAt != nil {
		return out, ErrReferenceUnavailable.OnField("source_id")
	}

	out.id = current.ID

	var last ticketState
	_ = json.Unmarshal(s.prev.State, &last)

	want, ok := w.Mapping.States[t.stateKey(t.StateRef)]
	if !ok || want == current.StateID {
		out.state = mustJSON(ticketState{StateID: current.StateID})

		return out, nil
	}

	if last.StateID != ids.Nil() && last.StateID != current.StateID {
		out.conflicts = append(out.conflicts, Conflict{
			Field: "state_ref", Dimmi: models.PublicID(models.PrefixTicketState, current.StateID),
			Source: strings.TrimPrefix(t.StateRef, ""),
		})
		out.state = s.prev.State

		return out, nil
	}

	tw, err := w.TicketWriter(ctx, s.tx, timeOr(t.UpdatedAt, s.now))
	if err != nil {
		return out, err
	}

	if !tw.Setup.Allows(current.TypeID, want) {
		return out, nil
	}

	moved, err := tw.Apply(ctx, current, tickets.Change{StateID: &want})
	if err != nil {
		return out, err
	}

	out.state = mustJSON(ticketState{StateID: moved.StateID})

	return out, nil
}
