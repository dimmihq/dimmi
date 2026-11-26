package intercom

import "encoding/json"

type attachment struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	ContentType string `json:"content_type"`
	Filesize    int64  `json:"filesize"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
}

type author struct {
	Type        string `json:"type"`
	ID          ID     `json:"id"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	FromAIAgent bool   `json:"from_ai_agent"`
	IsAIAnswer  bool   `json:"is_ai_answer"`
}

type part struct {
	ID              ID              `json:"id"`
	PartType        string          `json:"part_type"`
	Body            string          `json:"body"`
	CreatedAt       Time            `json:"created_at"`
	UpdatedAt       Time            `json:"updated_at"`
	AssignedTo      *ref            `json:"assigned_to"`
	Author          author          `json:"author"`
	Attachments     []attachment    `json:"attachments"`
	Redacted        bool            `json:"redacted"`
	State           string          `json:"state"`
	Tags            []tag           `json:"tags"`
	EventDetails    json.RawMessage `json:"event_details"`
	EmailMetadata   *emailMeta      `json:"email_message_metadata"`
	PreviousState   string          `json:"previous_ticket_state"`
	TicketState     string          `json:"ticket_state"`
	UpdatedAttrData json.RawMessage `json:"updated_attribute_data"`
}

type emailMeta struct {
	MessageID string `json:"message_id"`
	Subject   string `json:"subject"`
}

type conversation struct {
	ID              ID             `json:"id"`
	Title           string         `json:"title"`
	CreatedAt       Time           `json:"created_at"`
	UpdatedAt       Time           `json:"updated_at"`
	SnoozedUntil    Time           `json:"snoozed_until"`
	State           string         `json:"state"`
	Priority        string         `json:"priority"`
	AdminAssigneeID ID             `json:"admin_assignee_id"`
	TeamAssigneeID  ID             `json:"team_assignee_id"`
	CustomAttrs     map[string]any `json:"custom_attributes"`
	Tags            struct {
		Tags []tag `json:"tags"`
	} `json:"tags"`
	Rating *struct {
		Rating    int    `json:"rating"`
		Remark    string `json:"remark"`
		CreatedAt Time   `json:"created_at"`
		Teammate  *ref   `json:"teammate"`
	} `json:"conversation_rating"`
	Source struct {
		Type          string       `json:"type"`
		ID            ID           `json:"id"`
		DeliveredAs   string       `json:"delivered_as"`
		Subject       string       `json:"subject"`
		Body          string       `json:"body"`
		Author        author       `json:"author"`
		Attachments   []attachment `json:"attachments"`
		URL           string       `json:"url"`
		Redacted      bool         `json:"redacted"`
		EmailMetadata *emailMeta   `json:"email_message_metadata"`
	} `json:"source"`
	Contacts struct {
		Contacts []ref `json:"contacts"`
	} `json:"contacts"`
	Company *struct {
		ID ID `json:"id"`
	} `json:"company"`
	Parts struct {
		Parts      []part `json:"conversation_parts"`
		TotalCount int    `json:"total_count"`
	} `json:"conversation_parts"`
}

type ticketState struct {
	ID            ID     `json:"id"`
	Category      string `json:"category"`
	InternalLabel string `json:"internal_label"`
	ExternalLabel string `json:"external_label"`
}

type ticketTypeAttr struct {
	ID                ID     `json:"id"`
	Name              string `json:"name"`
	Description       string `json:"description"`
	DataType          string `json:"data_type"`
	Default           bool   `json:"default"`
	VisibleToContacts bool   `json:"visible_to_contacts"`
	RequiredToCreate  bool   `json:"required_to_create"`
	Archived          bool   `json:"archived"`
	Input             struct {
		Multiline bool `json:"multiline"`
		Options   []struct {
			Label    string `json:"label"`
			Archived bool   `json:"archived"`
		} `json:"list_options"`
	} `json:"input_options"`
}

type ticketType struct {
	ID          ID     `json:"id"`
	Category    string `json:"category"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Archived    bool   `json:"archived"`
	UpdatedAt   Time   `json:"updated_at"`
	Attributes  struct {
		Data []ticketTypeAttr `json:"ticket_type_attributes"`
	} `json:"ticket_type_attributes"`
	States struct {
		Data []ticketState `json:"data"`
	} `json:"ticket_states"`
}

type ticket struct {
	ID              ID             `json:"id"`
	TicketID        string         `json:"ticket_id"`
	Category        string         `json:"category"`
	Attributes      map[string]any `json:"ticket_attributes"`
	State           ticketState    `json:"ticket_state"`
	Type            ticketType     `json:"ticket_type"`
	AdminAssigneeID ID             `json:"admin_assignee_id"`
	TeamAssigneeID  ID             `json:"team_assignee_id"`
	CreatedAt       Time           `json:"created_at"`
	UpdatedAt       Time           `json:"updated_at"`
	Open            bool           `json:"open"`
	SnoozedUntil    Time           `json:"snoozed_until"`
	CompanyID       ID             `json:"company_id"`
	Contacts        struct {
		Contacts []ref `json:"contacts"`
	} `json:"contacts"`
	Linked struct {
		Data []ref `json:"data"`
	} `json:"linked_objects"`
	Parts struct {
		Parts      []part `json:"ticket_parts"`
		TotalCount int    `json:"total_count"`
	} `json:"ticket_parts"`
	Tags struct {
		Tags []tag `json:"tags"`
	} `json:"tags"`
}
