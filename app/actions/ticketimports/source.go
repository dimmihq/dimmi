/*
|--------------------------------------------------------------------------
| Import Sources
|--------------------------------------------------------------------------
|
| Each tool is read into the same plain shape: a catalog of types with
| their fields and the states they use, then the tickets one at a time,
| each with its requester, values, the state it is in and the states it
| went through. Ids are the other tool's, so a rerun finds what an
| earlier one made. A ticket may name the tracker it reports, and the
| conversation it came from when that was imported too.
|
*/

package ticketimports

import (
	"context"
	"time"
)

/*
Source reads one tool's tickets. Name is intercom or zendesk.
*/
type Source interface {
	// Name names the tool.
	Name() string
	// Catalog reads how the tool sets tickets up.
	Catalog(ctx context.Context) (Catalog, error)
	// Tickets hands every ticket to the function, oldest first.
	Tickets(ctx context.Context, each func(Ticket) error) error
}

/*
Catalog is how the tool sets tickets up.
*/
type Catalog struct {
	Types  []Type
	States []State
}

/*
Type is a ticket type: its kind (customer, back_office or tracker),
its fields, and the external ids of the states it uses, the first its
default.
*/
type Type struct {
	ExternalID  string
	Kind        string
	Name        string
	Description string
	Fields      []Field
	States      []string
}

/*
Field is one field of a type, its data type one tickets accept and
its options for a choice.
*/
type Field struct {
	ExternalID string
	Key        string
	Label      string
	DataType   string
	Options    []string
	Customer   bool
}

/*
State is a state with its group and the label customers saw.
*/
type State struct {
	ExternalID    string
	Group         string
	Name          string
	CustomerLabel string
}

/*
Ticket is one ticket. Values are by field key. History is the states
it went through, oldest first, the first the one it was created in;
StateID is the one it is in now.
*/
type Ticket struct {
	ExternalID     string
	TypeID         string
	StateID        string
	Title          string
	Description    string
	Values         map[string]any
	Priority       string
	RequesterEmail string
	RequesterName  string
	AssigneeEmail  string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	History        []Move
	TrackerID      string
	Conversation   string
}
