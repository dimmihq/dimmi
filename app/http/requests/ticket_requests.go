/*
|--------------------------------------------------------------------------
| Ticket Requests
|--------------------------------------------------------------------------
|
| The inputs of tickets: a filtered page, a new ticket, one ticket, and
| a change to one. A create with an external id the workspace already
| used returns the first ticket. A change sent with If-Match, or with a
| version, is refused when the ticket moved on since.
|
*/

package requests

/*
TicketsRequest filters a page of tickets. Fields is a JSON object of
field values to match exactly.
*/
type TicketsRequest struct {
	TypeID         string `query:"type_id"         maxLength:"640" doc:"Type ids, separated by commas."`
	Kind           string `query:"kind"            maxLength:"80"  doc:"Kinds, separated by commas."`
	StateID        string `query:"state_id"        maxLength:"640" doc:"State ids, separated by commas."`
	Group          string `query:"group"           maxLength:"120" doc:"State groups, separated by commas."`
	Active         bool   `query:"active"          doc:"Only tickets not in a resolved state."`
	AssigneeID     string `query:"assignee_id"     maxLength:"64"  doc:"A teammate's id, or none for unassigned."`
	TeamID         string `query:"team_id"         maxLength:"64"`
	CompanyID      string `query:"company_id"      maxLength:"64"`
	ContactID      string `query:"contact_id"      maxLength:"64"`
	ConversationID string `query:"conversation_id" maxLength:"64"  doc:"Trackers linked to it."`
	Number         string `query:"number"          maxLength:"24"  doc:"A reference such as T-482."`
	ExternalID     string `query:"external_id"     maxLength:"255"`
	Priority       string `query:"priority"        enum:"none,low,medium,high,urgent"`
	Fields         string `query:"fields"          maxLength:"4000"`
	IssueStatus    string `query:"issue_status"    enum:"backlog,unstarted,started,done,canceled"`
	CreatedAfter   string `query:"created_after"   maxLength:"40"`
	CreatedBefore  string `query:"created_before"  maxLength:"40"`
	ResolvedAfter  string `query:"resolved_after"  maxLength:"40"`
	ResolvedBefore string `query:"resolved_before" maxLength:"40"`
	Q              string `query:"q"               maxLength:"200"`
	Trashed        bool   `query:"trashed"`
	Cursor         string `query:"cursor"          maxLength:"64"`
	Limit          int    `query:"limit"           minimum:"0"     maximum:"100"`
}

/*
TicketLinkBody is a link to another ticket or to a conversation.
*/
type TicketLinkBody struct {
	Kind           string  `json:"kind"                      enum:"back_office,tracker,follow_up,related" validate:"required"` //nolint:lll // one enum
	TicketID       *string `json:"ticket_id,omitempty"       maxLength:"64"`
	ConversationID *string `json:"conversation_id,omitempty" maxLength:"64"`
}

/*
StoreTicketRequest is the input of POST /tickets.
*/
type StoreTicketRequest struct {
	Body struct {
		TypeID         string           `json:"type_id"                   maxLength:"64"    validate:"required"`
		StateID        *string          `json:"state_id,omitempty"        maxLength:"64"`
		Title          string           `json:"title"                     maxLength:"200"   validate:"required"`
		Description    string           `json:"description,omitempty"     maxLength:"20000"`
		Fields         map[string]any   `json:"fields,omitempty"`
		ContactID      *string          `json:"contact_id,omitempty"      maxLength:"64"`
		CompanyID      *string          `json:"company_id,omitempty"      maxLength:"64"`
		ConversationID *string          `json:"conversation_id,omitempty" maxLength:"64"`
		AssigneeID     *string          `json:"assignee_id,omitempty"     maxLength:"64"`
		TeamID         *string          `json:"team_id,omitempty"         maxLength:"64"`
		InboxID        *string          `json:"inbox_id,omitempty"        maxLength:"64"`
		Priority       *string          `json:"priority,omitempty"        enum:"none,low,medium,high,urgent"`
		ExternalID     *string          `json:"external_id,omitempty"     maxLength:"255"`
		SLAPolicyID    *string          `json:"sla_policy_id,omitempty"   maxLength:"64"`
		LinkTo         []TicketLinkBody `json:"link_to,omitempty"         maxItems:"500"`
	}
}

/*
TicketRequest names a ticket in the path, by id or reference.
*/
type TicketRequest struct {
	Ticket string `path:"ticket"`
}

/*
UpdateTicketRequest is the input of PATCH /tickets/{ticket}. A null
assignee or team unassigns. Accept_suggestions keeps the AI's values
for the keys.
*/
type UpdateTicketRequest struct {
	Ticket  string `path:"ticket"`
	IfMatch string `header:"If-Match"`
	RawBody []byte
	Body    struct {
		Version           *int32         `json:"version,omitempty"`
		StateID           *string        `json:"state_id,omitempty"       maxLength:"64"`
		TypeID            *string        `json:"type_id,omitempty"        maxLength:"64"`
		Title             *string        `json:"title,omitempty"          maxLength:"200"`
		Description       *string        `json:"description,omitempty"    maxLength:"20000"`
		Fields            map[string]any `json:"fields,omitempty"`
		Priority          *string        `json:"priority,omitempty"       enum:"none,low,medium,high,urgent"`
		AssigneeID        *string        `json:"assignee_id,omitempty"    maxLength:"64"`
		TeamID            *string        `json:"team_id,omitempty"        maxLength:"64"`
		SLAPolicyID       *string        `json:"sla_policy_id,omitempty"  maxLength:"64"`
		AcceptSuggestions []string       `json:"accept_suggestions,omitempty" maxItems:"200"`
	}
}
